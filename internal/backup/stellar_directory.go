package backup

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ReadStellarContentDirectory 使用与 ZIP 相同的 manifest/source 协议。
// 仅读取 manifest 声明的相对路径，不跟随目录或文件符号链接。
func ReadStellarContentDirectory(root string) (StellarContentManifest, error) {
	var manifest StellarContentManifest
	raw, err := readStellarDirectoryFile(root, StellarContentManifestName)
	if err != nil {
		return manifest, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil || decoder.Decode(new(any)) != io.EOF {
		return manifest, fmt.Errorf("invalid content directory manifest")
	}
	sources, err := stellarContentSources(manifest)
	if err != nil {
		return manifest, err
	}
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	entry, err := writer.Create(StellarContentManifestName)
	if err != nil {
		return manifest, err
	}
	if _, err := entry.Write(raw); err != nil {
		_ = writer.Close()
		return manifest, err
	}
	for name := range sources {
		data, err := readStellarDirectoryFile(root, name)
		if err != nil {
			_ = writer.Close()
			return manifest, err
		}
		entry, err := writer.Create(name)
		if err != nil {
			_ = writer.Close()
			return manifest, err
		}
		if _, err := entry.Write(data); err != nil {
			_ = writer.Close()
			return manifest, err
		}
	}
	if err := writer.Close(); err != nil {
		return manifest, err
	}
	validated, _, err := ReadStellarContentZip(archive.Bytes())
	return validated, err
}

func readStellarDirectoryFile(root, source string) ([]byte, error) {
	if !validStellarContentPath(source) {
		return nil, fmt.Errorf("invalid content directory member")
	}
	current := root
	for _, component := range strings.Split(source, "/") {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("content directory member cannot be a symbolic link")
		}
	}
	return readRegularFile(current, maxZipEntrySize)
}
