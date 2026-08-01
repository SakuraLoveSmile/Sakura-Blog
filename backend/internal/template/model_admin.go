package template

// AdminDashboard 仪表盘聚合数据。
type AdminDashboard struct {
	PostTotal       int
	PostPublished   int
	CommentTotal    int
	CommentPending  int
	ViewTotal       int64
	AttachmentTotal int
	RecentComments  []*AdminCommentRow
}
