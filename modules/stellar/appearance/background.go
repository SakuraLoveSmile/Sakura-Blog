package appearance

import (
	"net/url"
	"strconv"
	"strings"
)

const decoration = ".site-region--leftbar .site-region__decoration"
const collapsedDecoration = "html[data-leftbar-state='collapsed'] .site-shell:not([data-drawer='leftbar']) .site-region--leftbar .site-region__decoration"

func background(b *strings.Builder, r *reader, preset string, base *url.URL) {
	kind := r.choice("appearance.backgrounds.leftbar.type", "none", "gradient", "image")
	image := resolvedResource(r.resource("appearance.backgrounds.leftbar.image"), base)
	light := r.colors("appearance.backgrounds.leftbar.gradient.light")
	dark := r.colors("appearance.backgrounds.leftbar.gradient.dark")
	opacity := r.number("appearance.backgrounds.leftbar.opacity", 0, 1)
	blur := r.length("appearance.backgrounds.leftbar.backdrop.radius")
	if preset != "glass" {
		return
	}
	if kind == "none" {
		rule(b, decoration, declarations{"display": "none", "background-image": "none"})
		return
	}
	rule(b, decoration, declarations{
		"display": "block", "pointer-events": "none", "position": "absolute", "--inset": "16px", "top": "var(--inset)", "bottom": "var(--inset)", "left": "var(--inset)", "right": "var(--inset)", "border-radius": "inherit", "--saturate": "350%", "--blur-px": blur,
	})
	usesImage := kind == "image" && image != ""
	material := func(colors []string, dark bool) declarations {
		d := declarations{"--background-opacity": num(opacity), "background-repeat": "no-repeat"}
		if usesImage {
			if dark {
				d["--background-opacity"] = num(opacity * .75)
			}
			d["filter"] = "saturate(var(--saturate)) blur(var(--blur-px)) opacity(var(--background-opacity))"
			d["background-color"] = "transparent"
			d["background-image"] = cssURL(image)
			d["background-position"] = "center"
			d["background-size"] = "cover"
		} else if len(colors) == 4 {
			d["filter"] = "blur(var(--blur-px)) opacity(var(--background-opacity))"
			d["background-color"] = colors[0]
			d["background-image"] = "radial-gradient(ellipse at 28% 24%, " + colors[1] + " 0%, " + colors[1] + " 65%, transparent 100%), radial-gradient(ellipse at 60% 50%, " + colors[2] + " 0%, " + colors[2] + " 60%, transparent 100%), conic-gradient(from 210deg at 32% 72%, " + colors[3] + " 0deg, " + colors[3] + " 110deg, transparent 190deg, " + colors[1] + " 270deg, transparent 360deg)"
			d["background-position"] = "left top, right 42%, left bottom"
			d["background-size"] = "56% 34%, 52% 30%, 60% 38%"
		}
		return d
	}
	rule(b, decoration, material(light, false))
	rule(b, ":root[data-theme='dark'] "+decoration, material(dark, true))
	b.WriteString("@media screen and (prefers-color-scheme:dark){")
	rule(b, ":root:not([data-theme]) "+decoration, material(dark, true))
	b.WriteByte('}')
	b.WriteString("@media screen and (min-width:769px){")
	rule(b, collapsedDecoration, declarations{"--inset": "0px", "--blur-px": halfLength(blur)})
	b.WriteByte('}')
}
func halfLength(value string) string {
	if value == "0" {
		return value
	}
	i := 0
	for i < len(value) && ((value[i] >= '0' && value[i] <= '9') || value[i] == '.') {
		i++
	}
	n, e := strconv.ParseFloat(value[:i], 64)
	if e != nil {
		return value
	}
	return num(n/2) + value[i:]
}
