package api

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// AttachmentStorage is the Console attachment persistence boundary.
// Save stores the main file, Thumb stores its derived thumbnail, and Remove
// removes either path. Paths are always relative to the implementation root.
type AttachmentStorage interface {
	Save(relativePath string, data []byte) error
	Thumb(relativePath string, data []byte) error
	Remove(relativePath string) error
}

type localAttachmentStorage struct {
	root string
}

// NewLocalAttachmentStorage is the only storage implementation currently used
// by the application. Its interface leaves room for S3/OSS without leaking
// filesystem paths into handlers.
func NewLocalAttachmentStorage(root string) AttachmentStorage {
	return &localAttachmentStorage{root: root}
}

func attachmentStorageOrLocal(storage AttachmentStorage, root string) AttachmentStorage {
	if storage != nil {
		return storage
	}
	return NewLocalAttachmentStorage(root)
}

func (s *localAttachmentStorage) path(relativePath string) (string, error) {
	if relativePath == "" || filepath.IsAbs(relativePath) || strings.Contains(relativePath, "..") {
		return "", errors.New("invalid attachment storage path")
	}
	return filepath.Join(s.root, filepath.FromSlash(relativePath)), nil
}

func (s *localAttachmentStorage) Save(relativePath string, data []byte) error {
	return s.write(relativePath, data)
}

func (s *localAttachmentStorage) Thumb(relativePath string, data []byte) error {
	return s.write(relativePath, data)
}

func (s *localAttachmentStorage) write(relativePath string, data []byte) error {
	path, err := s.path(relativePath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (s *localAttachmentStorage) Remove(relativePath string) error {
	path, err := s.path(relativePath)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
