package api

import (
	"strings"

	attachmentstorage "blog/internal/storage"
)

// AttachmentStorage remains an API-package alias for existing handler tests
// and extension code; implementations live in internal/storage.
type AttachmentStorage = attachmentstorage.AttachmentStorage
type AttachmentURLProvider = attachmentstorage.URLProvider

func NewLocalAttachmentStorage(root string) AttachmentStorage {
	return attachmentstorage.NewLocalAttachmentStorage(root)
}

func attachmentStorageOrLocal(storage AttachmentStorage, root string) AttachmentStorage {
	if storage != nil {
		return storage
	}
	return NewLocalAttachmentStorage(root)
}

func (h *Handler) attachmentURL(relativePath string) string {
	if provider, ok := h.storage.(AttachmentURLProvider); ok {
		return provider.URL(relativePath)
	}
	return "/uploads/" + strings.TrimLeft(strings.ReplaceAll(relativePath, "\\", "/"), "/")
}
