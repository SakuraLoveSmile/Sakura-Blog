package template

// SiteStats 站点统计 widget 数据（文章/标签/评论/总阅读量）。
type SiteStats struct {
	Posts    int
	Tags     int
	Comments int
	Views    int64
}

// TagCloudFontSize 按标签文章数映射字号（12–22px），count 相同时取中间值。
func TagCloudFontSize(count, min, max int) int {
	const minPx, maxPx = 12, 22
	if count <= 0 {
		return minPx
	}
	if max <= min {
		return (minPx + maxPx) / 2
	}
	ratio := float64(count-min) / float64(max-min)
	return minPx + int(ratio*float64(maxPx-minPx))
}
