package routes

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"blog/internal/stellarv2"

	"github.com/gin-gonic/gin"
)

var gistIdentity = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9]+$`)

// 官方脚本使用 document.write，因此必须在单独文档的解析期加载。
// sandbox 的 opaque origin 使脚本不能读取父站 DOM、身份或存储。
var gistDocument = template.Must(template.New("gist-embed").Parse(`<!doctype html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<style nonce="{{.Nonce}}">html,body{margin:0;padding:0;background:transparent}body{overflow:hidden}.gist{margin:0!important}</style>
</head><body><script src="{{.Source}}"></script>
<script nonce="{{.Nonce}}" src="{{.Bridge}}"></script></body></html>`))

type gistDocumentData struct {
	Nonce  string
	Source string
	Bridge string
}

// gistEmbedHandler 只接受上游 owner/id 和 file，资源根地址来自可信主题配置。
// 不抓取用户 URL，也不将作者输入作为脚本正文或任意脚本地址。
func gistEmbedHandler(baseURL func() string) gin.HandlerFunc {
	return func(c *gin.Context) {
		query, err := url.ParseQuery(c.Request.URL.RawQuery)
		if err != nil {
			c.String(http.StatusBadRequest, "invalid gist parameters")
			return
		}
		for key, values := range query {
			if (key != "gist" && key != "file") || len(values) != 1 {
				c.String(http.StatusBadRequest, "invalid gist parameters")
				return
			}
		}
		gist := query.Get("gist")
		if !gistIdentity.MatchString(gist) {
			c.String(http.StatusBadRequest, "expected gist owner/id")
			return
		}
		base, err := url.Parse(strings.TrimRight(baseURL(), "/"))
		if err != nil || base == nil || (base.Scheme != "https" && base.Scheme != "http") || base.Host == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
			c.String(http.StatusInternalServerError, "invalid configured gist URL")
			return
		}
		base.Path = strings.TrimRight(base.Path, "/") + "/" + gist + ".js"
		base.RawPath = ""
		if file := query.Get("file"); file != "" {
			base.RawQuery = url.Values{"file": {file}}.Encode()
		}
		var random [18]byte
		if _, err := rand.Read(random[:]); err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		nonce := base64.RawStdEncoding.EncodeToString(random[:])
		origin := base.Scheme + "://" + base.Host
		var document bytes.Buffer
		if err := gistDocument.Execute(&document, gistDocumentData{Nonce: nonce, Source: base.String(), Bridge: strings.TrimSuffix(c.Request.URL.Path, "/stellar/embed/gist") + "/assets/js/blog-gist.js"}); err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Header("Content-Security-Policy", "default-src 'none'; script-src 'nonce-"+nonce+"' "+origin+"; style-src 'unsafe-inline' https:; img-src https: data:; font-src https:; base-uri 'none'; form-action 'none'; frame-ancestors 'self'; sandbox allow-scripts")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Cache-Control", "no-store")
		c.Data(http.StatusOK, "text/html; charset=utf-8", document.Bytes())
	}
}

func (s *webServer) registerEmbedRoutes(r *gin.Engine) {
	r.GET("/stellar/embed/gist", s.serveGistEmbed)
}
func (s *webServer) serveGistEmbed(c *gin.Context) {
	site := s.site.Load()
	if site == nil || site.StellarV2 == nil {
		c.String(http.StatusConflict, "Stellar v2 migration required")
		return
	}
	value, _ := stellarv2.Get(site.StellarV2.Theme, "services.github.gistUrl")
	base, _ := value.(string)
	gistEmbedHandler(func() string { return base })(c)
}
