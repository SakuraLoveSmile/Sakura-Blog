// Package stellarimages 实现 Stellar 2.0 的图片元信息生命周期。
package stellarimages

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
)

const (
	ColorAlgorithm = "hsla-mean-v1"
	SampleSize     = 64
	MaxBytes       = 32 * 1024 * 1024
	MaxPixels      = 40000000
)

type Dimensions struct {
	Status string `json:"status"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}
type Color struct {
	Status    string    `json:"status"`
	Algorithm string    `json:"algorithm"`
	Size      int       `json:"size,omitempty"`
	HSLA      []float64 `json:"hsla,omitempty"`
}
type Analysis struct {
	Dimensions *Dimensions `json:"dimensions,omitempty"`
	Color      *Color      `json:"color,omitempty"`
}
type Entry struct {
	UpdatedAt  string      `json:"updatedAt"`
	Dimensions *Dimensions `json:"dimensions,omitempty"`
	Color      *Color      `json:"color,omitempty"`
	Error      string      `json:"error,omitempty"`
	RetryAfter int64       `json:"retryAfter,omitempty"`
}
type Store struct {
	Version int              `json:"version"`
	Images  map[string]Entry `json:"images"`
}

func dimensionsReady(d *Dimensions) bool {
	return d != nil && d.Status == "ready" && d.Width > 0 && d.Height > 0
}
func colorReady(c *Color) bool {
	if c == nil || c.Status != "ready" || c.Algorithm != ColorAlgorithm || c.Size != SampleSize || len(c.HSLA) != 4 {
		return false
	}
	for i, v := range c.HSLA {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > []float64{360, 100, 100, 1}[i] {
			return false
		}
	}
	return true
}
func complete(e Entry) bool {
	return (dimensionsReady(e.Dimensions) || e.Dimensions != nil && e.Dimensions.Status == "unsupported") && (colorReady(e.Color) || e.Color != nil && e.Color.Status == "unsupported" && e.Color.Algorithm == ColorAlgorithm)
}

// ReadStore 对损坏或未知版本报错；调用者不得用空缓存覆盖原文件。
func ReadStore(path string) (Store, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Store{Version: 1, Images: map[string]Entry{}}, nil
	}
	if err != nil {
		return Store{}, fmt.Errorf("cannot read image metadata: %w", err)
	}
	var s Store
	if err = json.Unmarshal(b, &s); err != nil {
		return Store{}, fmt.Errorf("cannot read image metadata: %w", err)
	}
	if s.Version != 1 || s.Images == nil {
		return Store{}, errors.New("unrecognized image metadata format")
	}
	return s, nil
}

func writeStore(path string, s Store) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(b); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(name, path)
}
