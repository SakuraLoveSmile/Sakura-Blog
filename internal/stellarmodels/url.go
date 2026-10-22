package stellarmodels

import (
	"golang.org/x/net/idna"
	"net"
	"net/url"
	"strconv"
	"strings"
)

var originIDNA = idna.New(idna.MapForLookup(), idna.Transitional(false), idna.StrictDomainName(false), idna.CheckHyphens(false), idna.CheckJoiners(true), idna.BidiRule(), idna.VerifyDNSLength(false))

// URL origin 只描述数据服务来源，不授予网络访问权限。公开配置已经经过
// Schema 校验；这里仍需与 WHATWG 的 special URL 规范化保持一致。
func origin(value any) string {
	s := strings.TrimFunc(str(value), func(r rune) bool { return r <= 0x20 })
	s = strings.NewReplacer("\t", "", "\r", "", "\n", "").Replace(s)
	colon := strings.IndexByte(s, ':')
	if colon < 0 {
		return ""
	}
	scheme := strings.ToLower(s[:colon])
	switch scheme {
	case "http", "https", "ftp", "ws", "wss":
	default:
		return ""
	}
	rest := strings.TrimLeft(strings.ReplaceAll(s[colon+1:], "\\", "/"), "/")
	if end := strings.IndexAny(rest, "/?#"); end >= 0 {
		rest = rest[:end]
	}
	if at := strings.LastIndexByte(rest, '@'); at >= 0 {
		rest = rest[at+1:]
	}
	if rest == "" {
		return ""
	}
	host, port := rest, ""
	if strings.HasPrefix(host, "[") {
		end := strings.IndexByte(host, ']')
		if end < 0 {
			return ""
		}
		ip := net.ParseIP(host[1:end])
		if ip == nil {
			return ""
		}
		suffix := host[end+1:]
		host = "[" + ip.String() + "]"
		if suffix != "" {
			if suffix[0] != ':' {
				return ""
			}
			port = suffix[1:]
		}
	} else {
		if i := strings.LastIndexByte(host, ':'); i >= 0 {
			host, port = host[:i], host[i+1:]
		}
		decoded, e := url.PathUnescape(host)
		if e != nil {
			return ""
		}
		host, e = originIDNA.ToASCII(decoded)
		if e != nil || host == "" {
			return ""
		}
		host = strings.ToLower(host)
		if canonical, ok := ipv4Host(host); ok {
			host = canonical
		}
	}
	if port != "" {
		n, e := strconv.ParseUint(port, 10, 16)
		if e != nil {
			return ""
		}
		defaults := map[string]uint64{"http": 80, "https": 443, "ftp": 21, "ws": 80, "wss": 443}
		if n != defaults[scheme] {
			host += ":" + strconv.FormatUint(n, 10)
		}
	}
	return scheme + "://" + host
}
func ipv4Host(host string) (string, bool) {
	parts := strings.Split(strings.TrimSuffix(host, "."), ".")
	if len(parts) == 0 || len(parts) > 4 {
		return "", false
	}
	numbers := []uint64{}
	for _, part := range parts {
		base := 10
		digits := part
		if len(part) >= 2 && strings.EqualFold(part[:2], "0x") {
			base = 16
			digits = part[2:]
		} else if len(part) >= 2 && part[0] == '0' {
			base = 8
			digits = part[1:]
		}
		if digits == "" {
			digits = "0"
		}
		n, e := strconv.ParseUint(digits, base, 32)
		if e != nil {
			return "", false
		}
		numbers = append(numbers, n)
	}
	for _, n := range numbers[:len(numbers)-1] {
		if n > 255 {
			return "", false
		}
	}
	last := numbers[len(numbers)-1]
	if last >= uint64(1)<<(8*(5-len(numbers))) {
		return "", false
	}
	total := last
	for i, n := range numbers[:len(numbers)-1] {
		total += n << (8 * (3 - i))
	}
	return net.IPv4(byte(total>>24), byte(total>>16), byte(total>>8), byte(total)).String(), true
}
