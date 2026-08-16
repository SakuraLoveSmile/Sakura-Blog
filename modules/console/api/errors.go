package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"blog/internal/httpapi"
)

// Problem remains as a compatibility alias for existing Console callers.
type Problem = httpapi.Problem

// 预定义哨兵错误：RespondError 按 errors.Is 映射状态码。
// handler 可用 fmt.Errorf("...: %w", ErrNotFound) 包装携带语义。
var (
	ErrNotFound     = errors.New("not found")
	ErrForbidden    = errors.New("forbidden")
	ErrValidation   = errors.New("validation failed")
	ErrConflict     = errors.New("conflict")
	ErrUnauthorized = errors.New("unauthorized")
)

// problemType 按状态码推导机器可读类型标识（对标 RFC 9457 的 type 字段）。
func problemType(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "validation_error"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	case http.StatusTooManyRequests:
		return "rate_limited"
	case http.StatusServiceUnavailable:
		return "unavailable"
	default:
		return "internal_error"
	}
}

// RespondError 按哨兵错误类型映射状态码输出统一 Problem（未知类型回退 500）。
func RespondError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, ErrForbidden):
		status = http.StatusForbidden
	case errors.Is(err, ErrValidation):
		status = http.StatusBadRequest
	case errors.Is(err, ErrConflict):
		status = http.StatusConflict
	case errors.Is(err, ErrUnauthorized):
		status = http.StatusUnauthorized
	}
	RespondStatusError(c, status, err)
}

// RespondStatusError 显式状态码输出统一 Problem（error 字段与 detail 一致，向后兼容）。
func RespondStatusError(c *gin.Context, status int, err error) {
	if err == nil {
		err = errors.New(http.StatusText(status))
	}
	msg := err.Error()
	c.JSON(status, Problem{
		Type:     problemType(status),
		Title:    http.StatusText(status),
		Status:   status,
		Detail:   msg,
		Instance: c.Request.URL.Path,
		Error:    msg,
	})
}

// Fail 显式状态码 + 消息输出统一 Problem（现有 handler 批量迁移入口）。
func Fail(c *gin.Context, status int, msg string) {
	RespondStatusError(c, status, errors.New(msg))
}

// AbortFail 中间件场景用：输出统一 Problem 并中止请求链（不再执行后续 handler）。
func AbortFail(c *gin.Context, status int, msg string) {
	Fail(c, status, msg)
	c.Abort()
}
