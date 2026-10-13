package stellarhtml

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/idna"
)

var schemePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*:`)
var numericHost = regexp.MustCompile(`(?i)^(?:[0-9]+|0x[0-9a-f]*)$`)
var urlIDNA = idna.New(idna.MapForLookup(), idna.Transitional(false), idna.StrictDomainName(false), idna.CheckHyphens(false), idna.CheckJoiners(true), idna.BidiRule(), idna.VerifyDNSLength(false))

type httpURL struct {
	scheme, authority, path, query, fragment string
	hasQuery, hasFragment                    bool
}

func cleanURL(s string) string {
	return strings.NewReplacer("\t", "", "\r", "", "\n", "").Replace(strings.TrimFunc(s, func(r rune) bool { return r <= 0x20 }))
}
func resolveHTTPURL(href, base string) (string, error) {
	b, err := absoluteHTTP(cleanURL(base))
	if err != nil {
		return "", err
	}
	s := cleanURL(href)
	scheme := schemePattern.FindString(s)
	if scheme != "" {
		name := strings.ToLower(strings.TrimSuffix(scheme, ":"))
		if name != "http" && name != "https" {
			return "", fmt.Errorf("non-HTTP link")
		}
		rest := specialSlashes(s[len(scheme):])
		if name != b.scheme || strings.HasPrefix(rest, "//") {
			r, err := absoluteHTTP(name + ":" + rest)
			if err != nil {
				return "", err
			}
			return r.String(), nil
		}
		s = rest
	} else {
		s = specialSlashes(s)
	}
	if strings.HasPrefix(s, "//") {
		r, err := absoluteHTTP(b.scheme + ":" + s)
		if err != nil {
			return "", err
		}
		return r.String(), nil
	}
	path, query, fragment, hq, hf := splitSuffix(s)
	r := b
	r.hasFragment = hf
	r.fragment = percentEncode(fragment, "fragment")
	if path == "" {
		if hq {
			r.query = percentEncode(query, "query")
			r.hasQuery = true
		}
	} else {
		if !strings.HasPrefix(path, "/") {
			slash := strings.LastIndexByte(b.path, '/')
			path = b.path[:slash+1] + path
		}
		r.path = normalizePath(percentEncode(path, "path"))
		r.query = percentEncode(query, "query")
		r.hasQuery = hq
	}
	return r.String(), nil
}
func absoluteHTTP(s string) (httpURL, error) {
	var r httpURL
	scheme := schemePattern.FindString(s)
	r.scheme = strings.ToLower(strings.TrimSuffix(scheme, ":"))
	if r.scheme != "http" && r.scheme != "https" {
		return r, fmt.Errorf("absolute HTTP SiteURL required")
	}
	rest := strings.TrimLeft(specialSlashes(s[len(scheme):]), "/")
	i := strings.IndexAny(rest, "/?#")
	authority := rest
	if i >= 0 {
		authority = rest[:i]
		rest = rest[i:]
	} else {
		rest = ""
	}
	a, err := canonicalAuthority(authority, r.scheme)
	if err != nil {
		return r, err
	}
	r.authority = a
	path, query, fragment, hq, hf := splitSuffix(rest)
	r.path = normalizePath(percentEncode(path, "path"))
	r.query = percentEncode(query, "query")
	r.fragment = percentEncode(fragment, "fragment")
	r.hasQuery = hq
	r.hasFragment = hf
	return r, nil
}
func (u httpURL) String() string {
	s := u.scheme + "://" + u.authority + u.path
	if u.hasQuery {
		s += "?" + u.query
	}
	if u.hasFragment {
		s += "#" + u.fragment
	}
	return s
}
func splitSuffix(s string) (path, query, fragment string, hq, hf bool) {
	if i := strings.IndexByte(s, '#'); i >= 0 {
		fragment = s[i+1:]
		s = s[:i]
		hf = true
	}
	if i := strings.IndexByte(s, '?'); i >= 0 {
		query = s[i+1:]
		s = s[:i]
		hq = true
	}
	return s, query, fragment, hq, hf
}

// special URL 只在 authority/path 中将反斜杠视为斜杠，query/hash 保留原值。
func specialSlashes(s string) string {
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		return strings.ReplaceAll(s[:i], "\\", "/") + s[i:]
	}
	return strings.ReplaceAll(s, "\\", "/")
}
func canonicalAuthority(s, scheme string) (string, error) {
	credentials := ""
	if i := strings.LastIndexByte(s, '@'); i >= 0 {
		userinfo := s[:i]
		s = s[i+1:]
		parts := strings.SplitN(userinfo, ":", 2)
		user := percentEncode(parts[0], "user")
		pass := ""
		if len(parts) > 1 {
			pass = percentEncode(parts[1], "user")
		}
		if user != "" || pass != "" {
			credentials = user
			if pass != "" {
				credentials += ":" + pass
			}
			credentials += "@"
		}
	}
	host, port := s, ""
	if strings.HasPrefix(s, "[") {
		end := strings.IndexByte(s, ']')
		if end < 0 {
			return "", fmt.Errorf("invalid IPv6 host")
		}
		ip := net.ParseIP(s[1:end])
		if ip == nil || !strings.Contains(s[1:end], ":") {
			return "", fmt.Errorf("invalid IPv6 host")
		}
		host = "[" + serializeIPv6(ip) + "]"
		suffix := s[end+1:]
		if suffix != "" {
			if suffix[0] != ':' {
				return "", fmt.Errorf("invalid port")
			}
			port = suffix[1:]
		}
	} else {
		if i := strings.LastIndexByte(s, ':'); i >= 0 {
			host = s[:i]
			port = s[i+1:]
		}
		decoded, err := url.PathUnescape(host)
		if err != nil {
			return "", err
		}
		host, err = urlIDNA.ToASCII(decoded)
		if err != nil {
			return "", err
		}
		host = strings.ToLower(host)
		if host == "" || strings.ContainsAny(host, "\x00 #/:<>?@[\\]^|%") {
			return "", fmt.Errorf("invalid URL host")
		}
		for _, c := range host {
			if c <= 0x20 || c == 0x7f {
				return "", fmt.Errorf("invalid URL host")
			}
		}
		parts := strings.Split(strings.TrimSuffix(host, "."), ".")
		if numericHost.MatchString(parts[len(parts)-1]) {
			var err error
			host, err = canonicalIPv4(parts)
			if err != nil {
				return "", err
			}
		}
	}
	if port != "" {
		for _, c := range port {
			if c < '0' || c > '9' {
				return "", fmt.Errorf("invalid URL port")
			}
		}
		n, err := strconv.ParseUint(strings.TrimLeft(port, "0"), 10, 16)
		if strings.TrimLeft(port, "0") == "" {
			n = 0
			err = nil
		}
		if err != nil {
			return "", err
		}
		if (scheme == "http" && n == 80) || (scheme == "https" && n == 443) {
			port = ""
		} else {
			port = strconv.FormatUint(n, 10)
		}
	}
	if host == "" {
		return "", fmt.Errorf("empty URL host")
	}
	if port != "" {
		host += ":" + port
	}
	return credentials + host, nil
}
func canonicalIPv4(parts []string) (string, error) {
	if len(parts) > 4 {
		return "", fmt.Errorf("invalid IPv4 URL")
	}
	var result uint64
	for i, s := range parts {
		base := 10
		if len(s) >= 2 && strings.EqualFold(s[:2], "0x") {
			s = s[2:]
			base = 16
		} else if len(s) >= 2 && s[0] == '0' {
			s = s[1:]
			base = 8
		}
		var n uint64
		var err error
		if s != "" {
			n, err = strconv.ParseUint(s, base, 64)
		}
		limit := uint64(256)
		if i == len(parts)-1 {
			limit = uint64(1) << uint(8*(5-len(parts)))
		}
		if err != nil || n >= limit || parts[i] == "" {
			return "", fmt.Errorf("invalid IPv4 URL")
		}
		if i == len(parts)-1 {
			result += n
		} else {
			result += n << uint(8*(3-i))
		}
	}
	return fmt.Sprintf("%d.%d.%d.%d", byte(result>>24), byte(result>>16), byte(result>>8), byte(result)), nil
}

// WHATWG 将 IPv4 映射 IPv6 输出为十六进制组，不能使用 net.IP.String。
func serializeIPv6(ip net.IP) string {
	ip = ip.To16()
	groups := make([]string, 8)
	bestStart, bestLength := -1, 1
	for i := 0; i < 8; i++ {
		groups[i] = strconv.FormatUint(uint64(ip[i*2])<<8|uint64(ip[i*2+1]), 16)
		if groups[i] == "0" {
			j := i
			for j < 8 && ip[j*2] == 0 && ip[j*2+1] == 0 {
				j++
			}
			if j-i > bestLength {
				bestStart, bestLength = i, j-i
			}
		}
	}
	if bestStart < 0 {
		return strings.Join(groups, ":")
	}
	return strings.Join(groups[:bestStart], ":") + "::" + strings.Join(groups[bestStart+bestLength:], ":")
}
func normalizePath(s string) string {
	if s == "" {
		return "/"
	}
	if !strings.HasPrefix(s, "/") {
		s = "/" + s
	}
	parts := strings.Split(s[1:], "/")
	out := []string{}
	for i, part := range parts {
		dot := strings.ToLower(part)
		dot = strings.ReplaceAll(dot, "%2e", ".")
		switch dot {
		case ".":
			if i == len(parts)-1 {
				out = append(out, "")
			}
		case "..":
			if len(out) > 0 {
				out = out[:len(out)-1]
			}
			if i == len(parts)-1 {
				out = append(out, "")
			}
		default:
			out = append(out, part)
		}
	}
	return "/" + strings.Join(out, "/")
}
func percentEncode(s, kind string) string {
	var b strings.Builder
	const hex = "0123456789ABCDEF"
	for i := 0; i < len(s); i++ {
		c := s[i]
		encode := c <= 0x20 || c > 0x7e
		switch kind {
		case "path":
			encode = encode || strings.ContainsRune(`"#<>?`+"`{}", rune(c))
		case "query":
			encode = encode || strings.ContainsRune(`"#<>'`, rune(c))
		case "fragment":
			encode = encode || strings.ContainsRune(`"<>`+"`", rune(c))
		case "user":
			encode = encode || strings.ContainsRune(`"#<>?`+"`{} /:;=@[\\]^|", rune(c))
		}
		if encode {
			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&15])
		} else {
			b.WriteByte(c)
		}
	}
	return b.String()
}
func encodeURIComponent(s string) string {
	var b strings.Builder
	const hex = "0123456789ABCDEF"
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("-_.!~*'()", rune(c)) {
			b.WriteByte(c)
		} else {
			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&15])
		}
	}
	return b.String()
}
