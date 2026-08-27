package routes

import (
	"sync/atomic"

	tmpl "blog/modules/stellar/viewmodel"
)

// sitePtr 包装 *tmpl.Site 为原子指针，供路由测试构造 webServer。
func sitePtr(s *tmpl.Site) *atomic.Pointer[tmpl.Site] {
	p := &atomic.Pointer[tmpl.Site]{}
	p.Store(s)
	return p
}
