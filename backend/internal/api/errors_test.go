package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// doRequest 注册 handler 并执行一次请求，返回响应体 JSON map。
func doRequest(t *testing.T, handler gin.HandlerFunc) (int, map[string]any) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/test", handler)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v (body=%s)", err, w.Body.String())
	}
	return w.Code, body
}

// TestFailProblemShape 统一结构：Fail 输出 Problem 全部字段，且 error 字段保留（向后兼容）。
func TestFailProblemShape(t *testing.T) {
	status, body := doRequest(t, func(c *gin.Context) {
		Fail(c, http.StatusNotFound, "文章不存在")
	})
	if status != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", status)
	}
	for _, key := range []string{"type", "title", "status", "detail", "instance", "error"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("Problem missing field %q, body = %v", key, body)
		}
	}
	if body["type"] != "not_found" || body["title"] != "Not Found" {
		t.Fatalf("type/title wrong: %v", body)
	}
	if body["error"] != "文章不存在" || body["detail"] != "文章不存在" {
		t.Fatalf("error/detail should carry message: %v", body)
	}
	if body["instance"] != "/test" {
		t.Fatalf("instance should be request path: %v", body)
	}
}

// TestFailStatusTypes 不同状态码对应不同 type 标识。
func TestFailStatusTypes(t *testing.T) {
	cases := map[int]string{
		http.StatusBadRequest:          "validation_error",
		http.StatusUnauthorized:        "unauthorized",
		http.StatusForbidden:           "forbidden",
		http.StatusNotFound:            "not_found",
		http.StatusConflict:            "conflict",
		http.StatusTooManyRequests:     "rate_limited",
		http.StatusServiceUnavailable:  "unavailable",
		http.StatusInternalServerError: "internal_error",
		http.StatusBadGateway:          "internal_error",
	}
	for status, wantType := range cases {
		_, body := doRequest(t, func(c *gin.Context) {
			Fail(c, status, "x")
		})
		if body["type"] != wantType {
			t.Errorf("Fail(%d) type = %v, want %s", status, body["type"], wantType)
		}
		if int(body["status"].(float64)) != status {
			t.Errorf("Fail(%d) status field = %v", status, body["status"])
		}
	}
}

// TestRespondErrorSentinelMapping 哨兵错误按类型映射状态码。
func TestRespondErrorSentinelMapping(t *testing.T) {
	cases := map[error]int{
		ErrNotFound:         http.StatusNotFound,
		ErrForbidden:        http.StatusForbidden,
		ErrValidation:       http.StatusBadRequest,
		ErrConflict:         http.StatusConflict,
		ErrUnauthorized:     http.StatusUnauthorized,
		errors.New("random"): http.StatusInternalServerError,
	}
	for err, want := range cases {
		status, body := doRequest(t, func(c *gin.Context) {
			RespondError(c, err)
		})
		if status != want {
			t.Errorf("RespondError(%v) = %d, want %d", err, status, want)
		}
		if int(body["status"].(float64)) != want {
			t.Errorf("RespondError(%v) status field = %v", err, body["status"])
		}
	}
}

// TestRespondErrorWrapped 包装后的哨兵错误（%w）同样能映射。
func TestRespondErrorWrapped(t *testing.T) {
	status, _ := doRequest(t, func(c *gin.Context) {
		RespondError(c, fmt.Errorf("获取文章失败: %w", ErrNotFound))
	})
	if status != http.StatusNotFound {
		t.Fatalf("wrapped ErrNotFound should map to 404, got %d", status)
	}
}

// TestAbortFail 中间件场景：输出 Problem 并中止后续 handler。
func TestAbortFail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/test", AbortFailMiddleware(), func(c *gin.Context) {
		t.Fatal("handler after AbortFail must not run")
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if body["error"] != "模块未启用" || body["type"] != "forbidden" {
		t.Fatalf("problem body wrong: %v", body)
	}
}

// AbortFailMiddleware 测试用中间件：模拟 Gate 类拦截。
func AbortFailMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		AbortFail(c, http.StatusForbidden, "模块未启用")
	}
}
