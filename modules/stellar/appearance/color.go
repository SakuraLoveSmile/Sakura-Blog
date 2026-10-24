package appearance

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"golang.org/x/image/colornames"
)

type channels struct{ h, s, l, a float64 }

func clamp(v, max float64) float64 { return math.Max(0, math.Min(v, max)) }
func num(v float64) string {
	if math.Abs(v) < 1e-12 {
		v = 0
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}
func parseNumber(s string) (float64, bool) {
	v, e := strconv.ParseFloat(s, 64)
	return v, e == nil && !math.IsNaN(v) && !math.IsInf(v, 0)
}
func percent(s string) (float64, bool) {
	if !strings.HasSuffix(s, "%") {
		return 0, false
	}
	return parseNumber(strings.TrimSuffix(s, "%"))
}
func alpha(s string) (float64, bool) {
	if p, ok := percent(s); ok {
		return clamp(p/100, 1), true
	}
	v, ok := parseNumber(s)
	return clamp(v, 1), ok
}
func hue(s string) (float64, bool) {
	// 非 deg 单位交给浏览器相对色，避免复现 Stylus 把 turn/rad/grad 当 deg 的错误。
	if strings.HasSuffix(s, "turn") || strings.HasSuffix(s, "rad") || strings.HasSuffix(s, "grad") {
		return 0, false
	}
	s = strings.TrimSuffix(s, "deg")
	v, ok := parseNumber(s)
	return math.Mod(math.Mod(v, 360)+360, 360), ok
}
func fromRGB(r, g, b, a float64) channels {
	// 与 Stylus RGBA 同样截断范围并四舍五入到整数通道。
	r = math.Round(clamp(r, 255)) / 255
	g = math.Round(clamp(g, 255)) / 255
	b = math.Round(clamp(b, 255)) / 255
	lo := math.Min(r, math.Min(g, b))
	hi := math.Max(r, math.Max(g, b))
	d := hi - lo
	l := (hi + lo) / 2
	h, s := 0.0, 0.0
	if d != 0 {
		switch hi {
		case r:
			h = 60 * (g - b) / d
		case g:
			h = 60*(b-r)/d + 120
		default:
			h = 60*(r-g)/d + 240
		}
		if l < .5 {
			s = d / (2 * l)
		} else {
			s = d / (2 - 2*l)
		}
	}
	return channels{math.Mod(math.Mod(h, 360)+360, 360), s * 100, l * 100, clamp(a, 1)}
}

// parseColor 解析 Stylus 能正确转换的传统字面颜色；现代颜色和浏览器变量
// 保留为 CSS 相对色，不替换成默认色。
func parseColor(source string) (channels, bool) {
	s := strings.ToLower(source)
	if s == "transparent" {
		return channels{0, 0, 0, 0}, true
	}
	if s == "rebeccapurple" {
		return fromRGB(102, 51, 153, 1), true
	}
	if c, ok := colornames.Map[s]; ok {
		return fromRGB(float64(c.R), float64(c.G), float64(c.B), 1), true
	}
	if strings.HasPrefix(s, "#") {
		hex := s[1:]
		if len(hex) == 3 || len(hex) == 4 {
			var b strings.Builder
			for _, c := range hex {
				b.WriteRune(c)
				b.WriteRune(c)
			}
			hex = b.String()
		}
		if len(hex) != 6 && len(hex) != 8 {
			return channels{}, false
		}
		v, e := strconv.ParseUint(hex, 16, 32)
		if e != nil {
			return channels{}, false
		}
		a := 1.0
		if len(hex) == 8 {
			a = float64(v&255) / 255
			v >>= 8
		}
		return fromRGB(float64(v>>16), float64((v>>8)&255), float64(v&255), a), true
	}
	i := strings.IndexByte(s, '(')
	if i < 0 || !strings.HasSuffix(s, ")") {
		return channels{}, false
	}
	name := s[:i]
	if name != "hsl" && name != "hsla" && name != "rgb" && name != "rgba" {
		return channels{}, false
	}
	args := strings.Fields(strings.NewReplacer(",", " ", "/", " ").Replace(s[i+1 : len(s)-1]))
	if len(args) != 3 && len(args) != 4 {
		return channels{}, false
	}
	a := 1.0
	var ok bool
	if len(args) == 4 {
		a, ok = alpha(args[3])
		if !ok {
			return channels{}, false
		}
	}
	if name == "hsl" || name == "hsla" {
		h, ok := hue(args[0])
		if !ok {
			return channels{}, false
		}
		sv, ok := percent(args[1])
		if !ok {
			return channels{}, false
		}
		l, ok := percent(args[2])
		if !ok {
			return channels{}, false
		}
		return channels{h, clamp(sv, 100), clamp(l, 100), a}, true
	}
	rgb := [3]float64{}
	for j, arg := range args[:3] {
		if p, yes := percent(arg); yes {
			rgb[j] = p * 255 / 100
		} else {
			v, yes := parseNumber(arg)
			if !yes {
				return channels{}, false
			}
			rgb[j] = v
		}
	}
	return fromRGB(rgb[0], rgb[1], rgb[2], a), true
}
func colorHSLA(c channels, a string) string {
	return "hsla(" + num(c.h) + "deg " + num(c.s) + "% " + num(c.l) + "% / " + a + ")"
}
func relative(source, s, l, a string) string {
	return "hsl(from " + source + " h " + s + " " + l + " / " + a + ")"
}
func colorAlpha(source, a string) string {
	if c, ok := parseColor(source); ok {
		return colorHSLA(c, a)
	}
	return relative(source, "s", "l", a)
}

func colorLiteral(source string) string {
	if strings.EqualFold(source, "transparent") {
		return "transparent"
	}
	c, ok := parseColor(source)
	if !ok {
		return source
	}
	h, s, l := c.h/60, c.s/100, c.l/100
	v := (1 - math.Abs(2*l-1)) * s
	x, m := v*(1-math.Abs(math.Mod(h, 2)-1)), l-v/2
	r, g, b := 0.0, 0.0, 0.0
	switch {
	case h < 1:
		r, g = v, x
	case h < 2:
		r, g = x, v
	case h < 3:
		g, b = v, x
	case h < 4:
		g, b = x, v
	case h < 5:
		r, b = x, v
	default:
		r, b = v, x
	}
	r, g, b = math.Round((r+m)*255), math.Round((g+m)*255), math.Round((b+m)*255)
	if c.a == 1 {
		return fmt.Sprintf("#%02x%02x%02x", int(r), int(g), int(b))
	}
	return "rgba(" + num(r) + "," + num(g) + "," + num(b) + "," + num(math.Round(c.a*1000)/1000) + ")"
}
