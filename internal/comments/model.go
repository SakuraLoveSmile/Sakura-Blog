package comments

import "html/template"

// CommentItem is the public comment tree payload shared by SSR and JSON APIs.
// ContentHTML is sanitized before it reaches this model.
type CommentItem struct {
	ID          int64          `json:"id"`
	Nick        string         `json:"nick"`
	Link        string         `json:"link"`
	ContentHTML template.HTML  `json:"content_html"`
	IsAdmin     bool           `json:"is_admin"`
	Verified    bool           `json:"verified"`
	Avatar      string         `json:"avatar"`
	TimeAgo     string         `json:"time_ago"`
	Date        string         `json:"date"`
	Likes       int64          `json:"likes"`
	Boosts      []BoostItem    `json:"boosts"`
	BoostCount  int            `json:"boost_count"`
	CanBoost    bool           `json:"can_boost"`
	ReplyRef    string         `json:"reply_ref,omitempty"`
	Children    []*CommentItem `json:"children,omitempty"`
}

// BoostItem is a short response attached to a comment.
type BoostItem struct {
	Emoji  string `json:"emoji"`
	Text   string `json:"text"`
	Nick   string `json:"nick"`
	Avatar string `json:"avatar"`
	Mine   bool   `json:"mine"`
}
