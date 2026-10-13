// Package content contains shared content-domain services used by both HTTP modules.
package content

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// PostPreview is the short-lived editor snapshot used by the public draft preview page.
type PostPreview struct {
	StellarVersion int       `json:"stellar_version"`
	StellarConfig  string    `json:"stellar_config"`
	BlogExtensions string    `json:"blog_extensions"`
	Title          string    `json:"title"`
	ContentMD      string    `json:"content_md"`
	Excerpt        string    `json:"excerpt"`
	Cover          string    `json:"cover"`
	Topic          string    `json:"topic"`
	Slug           string    `json:"slug"`
	CreatedAt      time.Time `json:"-"`
}

type previewEntry struct {
	data    PostPreview
	expires time.Time
}

const previewTTL = 10 * time.Minute

var previews = struct {
	sync.Mutex
	items map[string]*previewEntry
}{items: make(map[string]*previewEntry)}

// CreatePostPreview stores a draft and returns its unguessable short-lived token.
func CreatePostPreview(p PostPreview) (string, error) {
	p.CreatedAt = time.Now()
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	previews.Lock()
	defer previews.Unlock()
	prunePreviewsLocked()
	previews.items[token] = &previewEntry{data: p, expires: time.Now().Add(previewTTL)}
	return token, nil
}

// GetPostPreview returns a draft when it exists and has not expired.
func GetPostPreview(token string) (PostPreview, bool) {
	previews.Lock()
	defer previews.Unlock()
	prunePreviewsLocked()
	e, ok := previews.items[token]
	if !ok {
		return PostPreview{}, false
	}
	return e.data, true
}

func prunePreviewsLocked() {
	now := time.Now()
	for token, entry := range previews.items {
		if now.After(entry.expires) {
			delete(previews.items, token)
		}
	}
}
