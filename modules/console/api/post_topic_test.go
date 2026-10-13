package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestPostTopicIDFlow(t *testing.T) {
	r, st, user, _ := newFullStellarEnv(t)
	cookie := authCookie(user)

	w := do2FA(r, http.MethodPost, "/api/v1/topics", `{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","name":"文章关联专栏","slug":"post-topic"}`, cookie)
	if w.Code != http.StatusCreated {
		t.Fatalf("create topic: status=%d body=%s", w.Code, w.Body.String())
	}
	var topic struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &topic); err != nil || topic.ID == 0 {
		t.Fatalf("decode topic: %+v err=%v", topic, err)
	}

	w = do2FA(r, http.MethodPost, "/api/v1/posts", `{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","slug":"topic-api-post","title":"关联文章","topic_id":`+itoa(topic.ID)+`,"topic":"旧名称","status":"draft"}`, cookie)
	if w.Code != http.StatusCreated {
		t.Fatalf("create post: status=%d body=%s", w.Code, w.Body.String())
	}
	var created struct {
		TopicID int64  `json:"topic_id"`
		Topic   string `json:"topic"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode post: %v", err)
	}
	if created.TopicID != topic.ID || created.Topic != "文章关联专栏" {
		t.Fatalf("created topic fields = %+v", created)
	}

	post, err := st.GetBySlug("topic-api-post")
	if err != nil || post == nil || post.TopicID != topic.ID || post.Topic != "文章关联专栏" {
		t.Fatalf("stored topic fields = %+v err=%v", post, err)
	}

	w = do2FA(r, http.MethodPut, "/api/v1/posts/topic-api-post", `{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","slug":"topic-api-post","title":"关联文章","topic_id":0,"status":"draft"}`, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("clear topic: status=%d body=%s", w.Code, w.Body.String())
	}
	post, _ = st.GetBySlug("topic-api-post")
	if post.TopicID != 0 || post.Topic != "" {
		t.Fatalf("topic was not cleared: %+v", post)
	}

	w = do2FA(r, http.MethodPut, "/api/v1/posts/topic-api-post", `{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","slug":"topic-api-post","title":"关联文章","topic_id":null,"status":"draft"}`, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("clear topic with null: status=%d body=%s", w.Code, w.Body.String())
	}

	w = do2FA(r, http.MethodPut, "/api/v1/posts/topic-api-post", `{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","slug":"topic-api-post","title":"关联文章","topic_id":99999,"status":"draft"}`, cookie)
	if w.Code != http.StatusNotFound {
		t.Errorf("invalid topic id: status=%d body=%s, want 404", w.Code, w.Body.String())
	}
	w = do2FA(r, http.MethodPut, "/api/v1/posts/topic-api-post", `{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","slug":"topic-api-post","title":"关联文章","topic_id":"bad","status":"draft"}`, cookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("malformed topic id: status=%d body=%s, want 400", w.Code, w.Body.String())
	}

	// 草稿和回收站文章都参与删除保护。
	w = do2FA(r, http.MethodPut, "/api/v1/posts/topic-api-post", `{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","slug":"topic-api-post","title":"关联文章","topic_id":`+itoa(topic.ID)+`,"status":"draft"}`, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("link draft: status=%d body=%s", w.Code, w.Body.String())
	}
	w = do2FA(r, http.MethodDelete, "/api/v1/posts/topic-api-post", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("trash post: status=%d body=%s", w.Code, w.Body.String())
	}
	w = do2FA(r, http.MethodDelete, "/api/v1/topics/"+itoa(topic.ID), "", cookie)
	if w.Code != http.StatusForbidden {
		t.Errorf("delete topic with trashed post: status=%d body=%s, want 403", w.Code, w.Body.String())
	}
}
