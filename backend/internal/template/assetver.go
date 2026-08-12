package template

// assetVersioner 资源版本号解析器，由启动侧注入（main.go 按 devMode 给磁盘/embed 实现）。
// 未注入时 AssetVersion 返回空串（模板输出 ?v= 空值，降级为无版本号）。
var assetVersioner func(string) string

// SetAssetVersioner 注入资源版本号解析器：入参为模板中的资源路径（如 /assets/css/main.css），
// 返回内容哈希版本后缀。根治手写 ?v= 版本号遗漏导致的浏览器旧缓存问题。
func SetAssetVersioner(fn func(string) string) { assetVersioner = fn }

// AssetVersion 模板函数 asset_ver 的实现。
func AssetVersion(path string) string {
	if assetVersioner == nil {
		return ""
	}
	return assetVersioner(path)
}
