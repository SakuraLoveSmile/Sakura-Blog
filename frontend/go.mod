// frontend module：仅容纳 assets/console 两个 go:embed 包（embed.go 必须与资源同目录）。
// 由根目录 go.work 与 backend module 关联，main.go 经 workspace 以 blog/frontend/... import 引用。
module blog/frontend

go 1.25.5
