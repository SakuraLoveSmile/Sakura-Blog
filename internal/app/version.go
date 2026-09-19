package app

var (
	// Version 由 release 构建经 ldflags 注入（tag 名，如 v1.4.2）；本地开发保持 dev。
	Version = "dev"
	// GitCommit 由构建时经 ldflags 注入（commit sha）；本地开发保持 dev。
	GitCommit = "dev"
	// BuildTime 由构建时经 ldflags 注入（RFC3339 时间戳）；本地开发保持 dev。
	BuildTime = "dev"
	// StellarCommit 由构建时经 ldflags 注入（Stellar 来源 commit）；本地开发保持 dev。
	StellarCommit = "dev"
	// FrontendFingerprint 由构建时经 ldflags 注入（前端构建指纹）；本地开发保持 dev。
	FrontendFingerprint = "dev"
)

// IsDev 判断当前是否为本地开发或未注入构建元数据的开发版本。
func IsDev() bool {
	return Version == "dev" || GitCommit == "dev"
}
