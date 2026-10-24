package content

import "fmt"

// StellarBodyRenderError 将可修复的正文错误与配置、归属及结构错误区分。
// Source 是已通过公开过滤的记录标识；原错误保留给日志与工具诊断。
type StellarBodyRenderError struct {
	Source string
	Err    error
}

func (e *StellarBodyRenderError) Error() string {
	return fmt.Sprintf("render stellar body %s: %v", e.Source, e.Err)
}
func (e *StellarBodyRenderError) Unwrap() error { return e.Err }
