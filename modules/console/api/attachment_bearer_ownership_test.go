package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
)

func bearerAttachmentRequest(r *gin.Engine, method, path, token string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	return w
}

func TestBearerAttachmentOwnershipAcrossConsoleAndUC(t *testing.T) {
	r, st, _, _ := newFullEnv(t)
	alice, err := st.GetUserByUsername("alice")
	if err != nil || alice == nil {
		t.Fatalf("get alice: %v", err)
	}
	bob, err := st.GetUserByUsername("bob")
	if err != nil || bob == nil {
		t.Fatalf("get bob: %v", err)
	}
	token := store.GenerateAPIToken()
	if err := st.SetAPITokenHash(alice.ID, store.HashAPIToken(token)); err != nil {
		t.Fatal(err)
	}
	create := func(owner int64, name string) int64 {
		id, err := st.CreateAttachment(&store.Attachment{Filename: name, Path: name, URL: "/uploads/" + name, Mime: "image/png", OwnerID: owner})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	aliceID := create(alice.ID, "alice.png")
	aliceSecondID := create(alice.ID, "alice-second.png")
	bobID := create(bob.ID, "bob.png")
	for _, path := range []string{"/api/v1/attachments", "/api/v1/uc/attachments"} {
		w := bearerAttachmentRequest(r, http.MethodGet, path, token)
		if w.Code != http.StatusOK {
			t.Fatalf("%s list status=%d body=%s", path, w.Code, w.Body.String())
		}
		var response struct {
			Items []struct {
				ID int64 `json:"id"`
			} `json:"items"`
			Total int `json:"total"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.Total != 2 || len(response.Items) != 2 {
			t.Fatalf("%s list=%+v, want only Alice's 2 attachments", path, response)
		}
		for _, item := range response.Items {
			if item.ID != aliceID && item.ID != aliceSecondID {
				t.Fatalf("list leaked another owner attachment %d", item.ID)
			}
		}
	}
	for _, path := range []string{"/api/v1/attachments/", "/api/v1/uc/attachments/"} {
		w := bearerAttachmentRequest(r, http.MethodDelete, path+itoa(bobID), token)
		if w.Code != http.StatusForbidden {
			t.Fatalf("%s cross-owner delete status=%d body=%s", path, w.Code, w.Body.String())
		}
	}
	w := bearerAttachmentRequest(r, http.MethodDelete, "/api/v1/attachments/"+itoa(aliceID), token)
	if w.Code != http.StatusOK {
		t.Fatalf("Console owned delete status=%d body=%s", w.Code, w.Body.String())
	}
	w = bearerAttachmentRequest(r, http.MethodDelete, "/api/v1/uc/attachments/"+itoa(aliceSecondID), token)
	if w.Code != http.StatusOK {
		t.Fatalf("UC owned delete status=%d", w.Code)
	}
	aliceSecond, _ := st.CountAttachmentsByOwner(alice.ID, "", "", nil)
	if aliceSecond != 0 {
		t.Fatalf("Alice attachment count after valid delete=%d, want 0", aliceSecond)
	}
}
