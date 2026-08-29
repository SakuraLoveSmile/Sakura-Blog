package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	dashboardLayoutGroup   = "dashboard-widgets"
	dashboardLayoutVersion = 10
	dashboardLayoutMaxBody = 512 << 10
	dashboardMaxWidgets    = 64
	dashboardMaxRows       = 64
)

var dashboardLayoutColumns = map[string]int{
	"lg":  12,
	"md":  12,
	"sm":  6,
	"xs":  4,
	"xxs": 4,
}

type dashboardLayoutWidget struct {
	I           string         `json:"i"`
	ID          string         `json:"id"`
	X           int            `json:"x"`
	Y           int            `json:"y"`
	W           int            `json:"w"`
	H           int            `json:"h"`
	MinW        *int           `json:"minW,omitempty"`
	MinH        *int           `json:"minH,omitempty"`
	MaxW        *int           `json:"maxW,omitempty"`
	MaxH        *int           `json:"maxH,omitempty"`
	Config      map[string]any `json:"config,omitempty"`
	Permissions []string       `json:"permissions,omitempty"`
}

type dashboardLayoutDocument struct {
	Version int                                `json:"version"`
	Layouts map[string][]dashboardLayoutWidget `json:"layouts"`
}

type dashboardLayoutResponse struct {
	Version   int                                 `json:"version"`
	Layouts   *map[string][]dashboardLayoutWidget `json:"layouts"`
	UpdatedAt string                              `json:"updated_at,omitempty"`
}

// getDashboardLayout GET /api/v1/dashboard/layout returns the current user's
// layout. A missing preference intentionally returns layouts=null so the SPA
// can choose its built-in defaults without coupling the API to Vue components.
func (h *Handler) getDashboardLayout(c *gin.Context) {
	preference, err := h.store.GetUserPreference(c.GetInt64("userID"), dashboardLayoutGroup)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if preference == nil {
		c.JSON(http.StatusOK, dashboardLayoutResponse{Version: dashboardLayoutVersion})
		return
	}

	document, err := decodeDashboardLayout(strings.NewReader(preference.Value))
	if err != nil {
		Fail(c, http.StatusInternalServerError, "已保存的 Dashboard 布局无效，请在 Designer 中重置")
		return
	}
	c.JSON(http.StatusOK, dashboardLayoutResponse{
		Version:   dashboardLayoutVersion,
		Layouts:   &document.Layouts,
		UpdatedAt: preference.UpdatedAt.UTC().Format(time.RFC3339),
	})
}

// saveDashboardLayout PUT /api/v1/dashboard/layout stores the current user's
// complete responsive layout after validating its bounded grid geometry.
func (h *Handler) saveDashboardLayout(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, dashboardLayoutMaxBody)
	document, err := decodeDashboardLayout(c.Request.Body)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	payload, err := json.Marshal(document)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "序列化 Dashboard 布局失败")
		return
	}
	if err := h.store.PutUserPreference(c.GetInt64("userID"), dashboardLayoutGroup, string(payload)); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, dashboardLayoutResponse{
		Version:   dashboardLayoutVersion,
		Layouts:   &document.Layouts,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	})
}

func decodeDashboardLayout(reader io.Reader) (*dashboardLayoutDocument, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var document dashboardLayoutDocument
	if err := decoder.Decode(&document); err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("dashboard 布局不能为空")
		}
		return nil, fmt.Errorf("无效的 Dashboard 布局 JSON")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("dashboard 布局只能包含一个 JSON 对象")
	}
	if document.Version != dashboardLayoutVersion {
		return nil, fmt.Errorf("不支持的 Dashboard 布局版本 %d", document.Version)
	}
	if document.Layouts == nil {
		return nil, fmt.Errorf("dashboard 布局缺少 layouts")
	}
	if len(document.Layouts) > len(dashboardLayoutColumns) {
		return nil, fmt.Errorf("dashboard 布局包含未知断点")
	}

	for breakpoint, widgets := range document.Layouts {
		columns, ok := dashboardLayoutColumns[breakpoint]
		if !ok {
			return nil, fmt.Errorf("未知 Dashboard 断点 %q", breakpoint)
		}
		if len(widgets) > dashboardMaxWidgets {
			return nil, fmt.Errorf("断点 %s 的组件数量超过限制", breakpoint)
		}
		seen := make(map[string]struct{}, len(widgets))
		for index := range widgets {
			widget := &widgets[index]
			if strings.TrimSpace(widget.I) == "" || strings.TrimSpace(widget.ID) == "" {
				return nil, fmt.Errorf("断点 %s 的第 %d 个组件缺少 i 或 id", breakpoint, index+1)
			}
			if _, exists := seen[widget.I]; exists {
				return nil, fmt.Errorf("断点 %s 存在重复组件实例 %q", breakpoint, widget.I)
			}
			seen[widget.I] = struct{}{}
			if widget.X < 0 || widget.Y < 0 || widget.W < 1 || widget.H < 1 || widget.X+widget.W > columns || widget.H > dashboardMaxRows {
				return nil, fmt.Errorf("断点 %s 的组件 %q 尺寸或位置越界", breakpoint, widget.I)
			}
			if err := validateDashboardConstraint(widget.MinW, widget.MaxW, widget.W, columns, "宽度", widget.I); err != nil {
				return nil, err
			}
			if err := validateDashboardConstraint(widget.MinH, widget.MaxH, widget.H, dashboardMaxRows, "高度", widget.I); err != nil {
				return nil, err
			}
			if len(widget.Permissions) > 32 {
				return nil, fmt.Errorf("组件 %q 的权限数量超过限制", widget.I)
			}
		}
		document.Layouts[breakpoint] = widgets
	}
	return &document, nil
}

func validateDashboardConstraint(minimum, maximum *int, current, limit int, label, instance string) error {
	if minimum != nil && (*minimum < 1 || *minimum > limit || *minimum > current) {
		return fmt.Errorf("组件 %q 的最小%s无效", instance, label)
	}
	if maximum != nil && (*maximum < 1 || *maximum > limit || *maximum < current) {
		return fmt.Errorf("组件 %q 的最大%s无效", instance, label)
	}
	return nil
}
