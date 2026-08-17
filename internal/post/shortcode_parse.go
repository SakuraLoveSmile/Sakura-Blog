package post

import (
	"regexp"
	"strings"
)

type markerPair struct {
	header string
	body   string
}

// splitByMarker 按 <!-- marker xxx --> 注释切分内容为 header/body 对（对齐 Stellar tabs.js / timeline.js）。
func splitByMarker(re *regexp.Regexp, content string) []markerPair {
	indices := re.FindAllStringSubmatchIndex(content, -1)
	if len(indices) == 0 {
		return nil
	}

	pairs := make([]markerPair, 0, len(indices))
	for i, loc := range indices {
		header := content[loc[2]:loc[3]]
		bodyStart := loc[1]
		bodyEnd := len(content)
		if i+1 < len(indices) {
			bodyEnd = indices[i+1][0]
		}
		pairs = append(pairs, markerPair{
			header: header,
			body:   content[bodyStart:bodyEnd],
		})
	}
	return pairs
}

// parseNamedPositionalArgs 解析 key:value 命名参数与位置参数（对齐 Stellar ctx.args.map）。
func parseNamedPositionalArgs(args string, namedKeys, positionalKeys []string) (map[string]string, []string) {
	namedSet := make(map[string]struct{}, len(namedKeys))
	for _, k := range namedKeys {
		namedSet[k] = struct{}{}
	}

	result := make(map[string]string)
	var positional []string

	for _, tok := range splitArgs(args) {
		isNamed := false
		for k := range namedSet {
			if strings.HasPrefix(tok, k+":") {
				result[k] = strings.TrimPrefix(tok, k+":")
				isNamed = true
				break
			}
		}
		if !isNamed {
			positional = append(positional, tok)
		}
	}

	for i, k := range positionalKeys {
		if i < len(positional) {
			result[k] = positional[i]
		}
	}
	return result, positional
}

// parseFoldingArgs 解析 folding 开标签参数：[color:xxx] [open:true/false] title
func parseFoldingArgs(args string) (color string, open bool, title string) {
	args = strings.TrimSpace(args)
	var remaining []string

	for _, tok := range splitArgs(args) {
		if strings.HasPrefix(tok, "color:") {
			color = strings.TrimPrefix(tok, "color:")
		} else if strings.HasPrefix(tok, "open:") {
			open = strings.TrimPrefix(tok, "open:") == "true"
		} else {
			remaining = append(remaining, tok)
		}
	}

	title = strings.Join(remaining, " ")
	return
}

// parseNoteArgs 解析 note 标签参数：[color:xxx] [title] content
func parseNoteArgs(args string) (color, title, content string) {
	args = strings.TrimSpace(args)
	var remaining []string

	for _, tok := range splitArgs(args) {
		if strings.HasPrefix(tok, "color:") {
			color = strings.TrimPrefix(tok, "color:")
		} else {
			remaining = append(remaining, tok)
		}
	}

	switch len(remaining) {
	case 0:
	case 1:
		content = remaining[0]
	default:
		title = remaining[0]
		content = strings.Join(remaining[1:], " ")
	}
	return
}

// splitArgs 按空白切分参数，但保留引号内的空格。
func splitArgs(s string) []string {
	var tokens []string
	var cur strings.Builder
	inQuote := false
	quoteChar := byte(0)

	for i := 0; i < len(s); i++ {
		c := s[i]
		if inQuote {
			if c == quoteChar {
				inQuote = false
			} else {
				cur.WriteByte(c)
			}
		} else if c == '"' || c == '\'' {
			inQuote = true
			quoteChar = c
		} else if c == ' ' || c == '\t' {
			if cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
			}
		} else {
			cur.WriteByte(c)
		}
	}
	if cur.Len() > 0 {
		tokens = append(tokens, cur.String())
	}
	return tokens
}
