package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
)

func decodeLinkJSON(t *testing.T, body []byte) linkJSON {
	t.Helper()
	var out linkJSON
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode link response: %v (%s)", err, body)
	}
	return out
}

// TestLinkLabelsCRUD 友链角标：最多 3 枚、首枚回写 label/color、附加角标随更新保留。
func TestLinkLabelsCRUD(t *testing.T) {
	r, st, user, _ := newFullEnv(t)
	cookie := authCookie(user)

	w := do2FA(r, http.MethodPost, "/api/v1/links",
		`{"name":"多角标","url":"https://multi.example","labels":[{"name":"社区先锋","color":"#D7EDE8"},{"name":"老友","color":"30B0C7"},{"name":"朋友","color":"007AFF"}]}`,
		cookie)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: status = %d body = %s", w.Code, w.Body.String())
	}
	created := decodeLinkJSON(t, w.Body.Bytes())
	if len(created.Labels) != 3 || created.Label != "社区先锋" || created.Color != "d7ede8" {
		t.Fatalf("created = %+v", created)
	}
	if created.Labels[1].Color != "30b0c7" || created.Labels[2].Name != "朋友" {
		t.Fatalf("created labels = %+v", created.Labels)
	}

	stored, err := st.GetLink(created.ID)
	if err != nil || stored == nil {
		t.Fatalf("get stored link: %v %v", stored, err)
	}
	if extras := stored.ExtraLabels(); len(extras) != 2 || extras[0].Name != "老友" {
		t.Fatalf("stored extras = %+v", extras)
	}

	// 更新为 2 枚：顺序与颜色按请求生效。
	w = do2FA(r, http.MethodPut, "/api/v1/links/"+strconv.FormatInt(created.ID, 10),
		`{"name":"多角标","url":"https://multi.example","labels":[{"name":"老友","color":"30B0C7"},{"name":"社区先锋","color":"D7EDE8"}]}`,
		cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("update: status = %d body = %s", w.Code, w.Body.String())
	}
	updated := decodeLinkJSON(t, w.Body.Bytes())
	if len(updated.Labels) != 2 || updated.Labels[0].Name != "老友" || updated.Label != "老友" {
		t.Fatalf("updated = %+v", updated)
	}

	// 4 枚 → 400。
	w = do2FA(r, http.MethodPut, "/api/v1/links/"+strconv.FormatInt(created.ID, 10),
		`{"name":"多角标","url":"https://multi.example","labels":[{"name":"一"},{"name":"二"},{"name":"三"},{"name":"四"}]}`,
		cookie)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("over limit: status = %d, want 400", w.Code)
	}

	// 文案过长 → 400。
	w = do2FA(r, http.MethodPut, "/api/v1/links/"+strconv.FormatInt(created.ID, 10),
		`{"name":"多角标","url":"https://multi.example","labels":[{"name":"这是一个非常非常长的角标文案"}]}`,
		cookie)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("long name: status = %d, want 400", w.Code)
	}

	// 旧客户端只传 label/color：保留既有附加角标。
	w = do2FA(r, http.MethodPut, "/api/v1/links/"+strconv.FormatInt(created.ID, 10),
		`{"name":"多角标","url":"https://multi.example","label":"老友","color":"30B0C7"}`,
		cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("legacy update: status = %d body = %s", w.Code, w.Body.String())
	}
	legacy := decodeLinkJSON(t, w.Body.Bytes())
	if len(legacy.Labels) != 2 || legacy.Labels[1].Name != "社区先锋" {
		t.Fatalf("legacy update dropped extras: %+v", legacy.Labels)
	}

	// 列表接口同样返回合并后的角标。
	w = do2FA(r, http.MethodGet, "/api/v1/links", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("list: status = %d", w.Code)
	}
	var listed struct {
		Items []linkJSON `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listed.Items) != 1 || len(listed.Items[0].Labels) != 2 {
		t.Fatalf("listed items = %+v", listed.Items)
	}

	// 清空角标：label/color 与附加角标一起清掉。
	w = do2FA(r, http.MethodPut, "/api/v1/links/"+strconv.FormatInt(created.ID, 10),
		`{"name":"多角标","url":"https://multi.example","labels":[]}`, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("clear: status = %d body = %s", w.Code, w.Body.String())
	}
	cleared := decodeLinkJSON(t, w.Body.Bytes())
	if len(cleared.Labels) != 0 || cleared.Label != "" || cleared.Color != "" {
		t.Fatalf("cleared = %+v", cleared)
	}
}
