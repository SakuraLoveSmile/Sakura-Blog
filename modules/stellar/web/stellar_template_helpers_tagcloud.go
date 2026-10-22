package web

// Color parsing and interpolation are a Go port of hexo-util 4.0.0's MIT
// Color helper. Attribution: stellar_template_helpers_testdata/LICENSE.hexo-util.

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"math"
	"math/big"
	"math/rand/v2"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

//go:embed stellar_template_helpers_colors.json
var stellarColorNamesJSON []byte
var stellarColorNames = func() map[string][4]float64 {
	var m map[string][4]float64
	if err := json.Unmarshal(stellarColorNamesJSON, &m); err != nil {
		panic(err)
	}
	return m
}()
var stellarRGBColor = regexp.MustCompile(`^rgba?\(\s*(\d{1,3})\s*,\s*(\d{1,3})\s*,\s*(\d{1,3})\s*,?\s*(0?\.?\d+)?\s*\)$`)
var stellarHSLColor = regexp.MustCompile(`^hsla?\(\s*(\d{1,3})\s*,\s*(\d{1,3})%\s*,\s*(\d{1,3})%\s*,?\s*(0?\.?\d+)?\s*\)$`)

func stellarFloat(value any, fallback float64) float64 {
	v, err := strconv.ParseFloat(stellarText(value), 64)
	if err != nil {
		return fallback
	}
	return v
}

// JS Number.toFixed rounds the exact binary value, rather than an already
// rounded value multiplied by 100 (which differs at values such as 1.115).
func stellarFixed(value float64, places int) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return value
	}
	negative := value < 0
	if negative {
		value = -value
	}
	scale := int64(math.Pow10(places))
	ratio := new(big.Rat).SetFloat64(value)
	ratio.Mul(ratio, new(big.Rat).SetInt64(scale))
	q, rem := new(big.Int), new(big.Int)
	q.QuoRem(ratio.Num(), ratio.Denom(), rem)
	if rem.Mul(rem, big.NewInt(2)).Cmp(ratio.Denom()) >= 0 {
		q.Add(q, big.NewInt(1))
	}
	n, _ := new(big.Rat).SetFrac(q, big.NewInt(scale)).Float64()
	if negative {
		n = -n
	}
	return n
}

func stellarParseColor(s string) ([4]float64, error) {
	s = strings.ToLower(s)
	c, ok := stellarColorNames[s]
	if ok {
		return c, nil
	}
	if len(s) == 4 && s[0] == '#' {
		s = fmt.Sprintf("#%c%c%c%c%c%c", s[1], s[1], s[2], s[2], s[3], s[3])
	}
	if len(s) == 7 && s[0] == '#' {
		if n, err := strconv.ParseUint(s[1:], 16, 24); err == nil {
			return [4]float64{float64(n >> 16), float64(n >> 8 & 255), float64(n & 255), 1}, nil
		}
	}
	if m := stellarRGBColor.FindStringSubmatch(s); m != nil {
		for i := 0; i < 3; i++ {
			c[i], _ = strconv.ParseFloat(m[i+1], 64)
		}
		c[3] = 1
		if m[4] != "" {
			c[3], _ = strconv.ParseFloat(m[4], 64)
		}
	} else if m := stellarHSLColor.FindStringSubmatch(s); m != nil {
		h, _ := strconv.ParseFloat(m[1], 64)
		sat, _ := strconv.ParseFloat(m[2], 64)
		light, _ := strconv.ParseFloat(m[3], 64)
		h /= 360
		sat /= 100
		light /= 100
		q := light + sat - light*sat
		if light < .5 {
			q = light * (1 + sat)
		}
		p := 2*light - q
		hue := func(v float64) float64 {
			if v < 0 {
				v++
			}
			if v > 1 {
				v--
			}
			color := p
			if v*6 < 1 {
				color = p + (q-p)*v*6
			} else if v*2 < 1 {
				color = q
			} else if v*3 < 2 {
				color = p + (q-p)*(2.0/3-v)*6
			}
			return math.Floor(color*255 + .5)
		}
		c = [4]float64{hue(h + 1.0/3), hue(h), hue(h - 1.0/3), 1}
		if m[4] != "" {
			c[3], _ = strconv.ParseFloat(m[4], 64)
		}
	} else {
		return c, fmt.Errorf("%s is not a supported color format", s)
	}
	if c[0] < 0 || c[0] > 255 || c[1] < 0 || c[1] > 255 || c[2] < 0 || c[2] > 255 || c[3] < 0 || c[3] > 1 {
		return c, fmt.Errorf("invalid RGBA color %s", s)
	}
	return c, nil
}
func stellarMixColor(a, b [4]float64, ratio float64) string {
	c := [4]float64{}
	for i := 0; i < 3; i++ {
		c[i] = math.Floor(a[i] + (b[i]-a[i])*ratio + .5)
	}
	c[3] = a[3] + (b[3]-a[3])*ratio
	if c[3] != 1 {
		return fmt.Sprintf("rgba(%d, %d, %d, %s)", int(c[0]), int(c[1]), int(c[2]), strconv.FormatFloat(stellarFixed(c[3], 2), 'f', -1, 64))
	}
	if int(c[0])%17 == 0 && int(c[1])%17 == 0 && int(c[2])%17 == 0 {
		return fmt.Sprintf("#%x%x%x", int(c[0])/17, int(c[1])/17, int(c[2])/17)
	}
	return fmt.Sprintf("#%02x%02x%02x", int(c[0]), int(c[1]), int(c[2]))
}

func (r stellarTemplateRenderer) tagcloud(d *StellarRenderData, item map[string]any) (template.HTML, error) {
	tags := []map[string]any{}
	for _, raw := range stellarList(d.Stellar.Data["tags"]) {
		tag := stellarMap(raw)
		tags = append(tags, map[string]any{"name": tag["name"], "path": tag["path"], "count": stellarInt(stellarFirst(tag["length"], tag["count"]), 0)})
	}
	if len(tags) == 0 {
		for _, tag := range d.TagCloud {
			tags = append(tags, map[string]any{"name": tag.Name, "path": "/tags/" + url.PathEscape(tag.Name) + "/", "count": tag.Count})
		}
	}
	if len(tags) == 0 {
		return "", nil
	}
	orderBy := stellarText(stellarFirst(item["orderby"], "name"))
	order := stellarInt(stellarFirst(item["order"], 1), 1)
	if orderBy == "random" || orderBy == "rand" {
		rand.Shuffle(len(tags), func(i, j int) { tags[i], tags[j] = tags[j], tags[i] })
	} else {
		sort.SliceStable(tags, func(i, j int) bool {
			less := false
			if orderBy == "length" || orderBy == "count" {
				less = stellarInt(tags[i]["count"], 0) < stellarInt(tags[j]["count"], 0)
			} else {
				less = stellarText(tags[i][orderBy]) < stellarText(tags[j][orderBy])
			}
			if order < 0 {
				if orderBy == "length" || orderBy == "count" {
					less = stellarInt(tags[i]["count"], 0) > stellarInt(tags[j]["count"], 0)
				} else {
					less = stellarText(tags[i][orderBy]) > stellarText(tags[j][orderBy])
				}
			}
			return less
		})
	}
	amount := stellarInt(item["amount"], 0)
	if amount > 0 && len(tags) > amount {
		tags = tags[:amount]
	} else if amount < 0 {
		n := len(tags) + amount
		if n < 0 {
			n = 0
		}
		tags = tags[:n]
	}
	counts := []int{}
	seen := map[int]bool{}
	for _, tag := range tags {
		count := stellarInt(tag["count"], 0)
		if !seen[count] {
			counts = append(counts, count)
			seen[count] = true
		}
	}
	sort.Ints(counts)
	var start, end [4]float64
	var err error
	color := stellarTruth(item["color"])
	if color {
		if !stellarTruth(item["start_color"]) {
			return "", fmt.Errorf("tagcloud start_color is required")
		}
		if !stellarTruth(item["end_color"]) {
			return "", fmt.Errorf("tagcloud end_color is required")
		}
		start, err = stellarParseColor(stellarText(item["start_color"]))
		if err != nil {
			return "", err
		}
		end, err = stellarParseColor(stellarText(item["end_color"]))
		if err != nil {
			return "", err
		}
	}
	minimum := stellarFloat(stellarFirst(item["min_font"], 10), 10)
	maximum := stellarFloat(stellarFirst(item["max_font"], 20), 20)
	unit := stellarText(stellarFirst(item["unit"], "px"))
	level := stellarFloat(stellarFirst(item["level"], 10), 10)
	separator := stellarText(stellarFirst(item["separator"], " "))
	countClass := stellarText(stellarFirst(item["count_class"], "count"))
	output := []string{}
	for _, tag := range tags {
		ratio := 0.0
		if len(counts) > 1 {
			ratio = float64(sort.SearchInts(counts, stellarInt(tag["count"], 0))) / float64(len(counts)-1)
		}
		font := stellarFixed(minimum+(maximum-minimum)*ratio, 2)
		style := "font-size: " + strconv.FormatFloat(font, 'f', -1, 64) + stellarCSSValue(unit) + ";"
		if color {
			style += " color: " + stellarMixColor(start, end, ratio)
		}
		body := stellarEscape(tag["name"])
		if stellarTruth(item["show_count"]) {
			body += `<span class="` + stellarEscape(countClass) + `">` + fmt.Sprint(stellarInt(tag["count"], 0)) + `</span>`
		}
		output = append(output, `<a href="`+stellarEscape(stellarURL(d.Stellar.Root, tag["path"]))+`" style="`+stellarEscape(style)+`" class="tag -`+fmt.Sprint(int(math.Floor(ratio*level+.5)))+`">`+body+`</a>`)
	}
	return r.frame(d, "tagcloud", stellarText(item["title"]), template.HTML(strings.Join(output, stellarEscape(separator))), "", "", nil, true), nil
}
