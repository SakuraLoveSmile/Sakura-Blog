package stellarimages

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// 使用合法公网地址，但所有请求只进入本地 transport，不做 DNS 或网络访问。
const remoteBoundaryURL = "https://93.184.216.34/original.png"

type remoteBoundaryTransport func(*http.Request) (*http.Response, error)

func (f remoteBoundaryTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type remoteBoundaryBody struct {
	io.Reader
	source    io.Closer
	readCalls int
	readBytes int
	closed    bool
}

func (b *remoteBoundaryBody) Read(p []byte) (int, error) {
	b.readCalls++
	n, err := b.Reader.Read(p)
	b.readBytes += n
	return n, err
}

func (b *remoteBoundaryBody) Close() error {
	b.closed = true
	if b.source != nil {
		return b.source.Close()
	}
	return nil
}

type remoteBoundaryZeros struct{}

func (remoteBoundaryZeros) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

type remoteBoundaryFixture struct {
	processor Processor
	original  string
	data      []byte
	cache     []byte
	old       Store
}

func newRemoteBoundaryFixture(t *testing.T) remoteBoundaryFixture {
	t.Helper()
	dir := t.TempDir()
	original := filepath.Join(dir, "original.png")
	data := fixture(t)
	if err := os.WriteFile(original, data, 0600); err != nil {
		t.Fatal(err)
	}
	old := Store{Version: 1, Images: map[string]Entry{
		remoteBoundaryURL: {
			UpdatedAt:  "old",
			Dimensions: &Dimensions{Status: "ready", Width: 7, Height: 11},
			Color: &Color{Status: "ready", Algorithm: ColorAlgorithm, Size: SampleSize,
				HSLA: []float64{10, 20, 30, 1}},
		},
		"https://93.184.216.34/other.png": {UpdatedAt: "untouched", Error: "old error", RetryAfter: 1},
	}}
	path := filepath.Join(dir, "images_metadata.json")
	if err := writeStore(path, old); err != nil {
		t.Fatal(err)
	}
	cache, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return remoteBoundaryFixture{
		processor: Processor{StorePath: path, SiteURL: "https://site.test/",
			Now: func() time.Time { return time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC) }},
		original: original, data: data, cache: cache, old: old,
	}
}

func (f remoteBoundaryFixture) assertOriginal(t *testing.T) {
	t.Helper()
	got, err := os.ReadFile(f.original)
	if err != nil || !bytes.Equal(got, f.data) {
		t.Fatalf("remote preparation changed original: %v", err)
	}
	if _, err = os.Stat(f.processor.StorePath + ".lock"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("preparation retained its lock: %v", err)
	}
}

func (f remoteBoundaryFixture) assertFailure(t *testing.T, report Report, err error, message string) {
	t.Helper()
	if err != nil || report.Found != 1 || report.Processed != 1 || report.Failed != 1 {
		t.Fatalf("expected reported image failure: %+v %v", report, err)
	}
	store, err := ReadStore(f.processor.StorePath)
	if err != nil {
		t.Fatal(err)
	}
	entry, old := store.Images[remoteBoundaryURL], f.old.Images[remoteBoundaryURL]
	if !strings.Contains(entry.Error, message) || entry.RetryAfter != f.processor.now().Add(24*time.Hour).UnixMilli() {
		t.Fatalf("missing failure/backoff: %+v", entry)
	}
	if !reflect.DeepEqual(entry.Dimensions, old.Dimensions) || !reflect.DeepEqual(entry.Color, old.Color) {
		t.Fatalf("failed download discarded old metadata: %+v", entry)
	}
	if !reflect.DeepEqual(store.Images["https://93.184.216.34/other.png"], f.old.Images["https://93.184.216.34/other.png"]) {
		t.Fatal("failed download changed unrelated metadata")
	}
	f.assertOriginal(t)
}

func TestRemoteDownloadSizeBoundaries(t *testing.T) {
	const limit = 32 * 1024 * 1024
	for _, tc := range []struct {
		name          string
		contentLength int64
		streamSize    int64
		wantBytes     int
		wantError     string
	}{
		{"declared too large", limit + 1, limit + 100, 0, "image exceeds download limit"},
		{"unknown length too large", -1, limit + 100, limit + 1, "image exceeds 33554432 bytes"},
		{"unknown length at limit", -1, limit, limit, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newRemoteBoundaryFixture(t)
			source, err := os.Open(f.original)
			if err != nil {
				t.Fatal(err)
			}
			defer source.Close()
			body := &remoteBoundaryBody{source: source,
				Reader: io.MultiReader(source, io.LimitReader(remoteBoundaryZeros{}, tc.streamSize-int64(len(f.data))))}
			f.processor.Client = &http.Client{Transport: remoteBoundaryTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.String() != remoteBoundaryURL || r.Method != http.MethodGet {
					return nil, errors.New("unexpected image request")
				}
				return &http.Response{StatusCode: http.StatusOK, Body: body, ContentLength: tc.contentLength}, nil
			})}
			report, err := f.processor.Prepare(context.Background(), nil, PrepareOptions{Refresh: remoteBoundaryURL})
			if body.readBytes != tc.wantBytes || !body.closed || tc.wantBytes == 0 && body.readCalls != 0 {
				t.Fatalf("download read %d bytes in %d calls, closed=%v", body.readBytes, body.readCalls, body.closed)
			}
			if tc.wantError != "" {
				f.assertFailure(t, report, err, tc.wantError)
				return
			}
			if err != nil || report.Processed != 1 || report.Failed != 0 {
				t.Fatalf("exact limit image rejected: %+v %v", report, err)
			}
			store, err := ReadStore(f.processor.StorePath)
			entry := store.Images[remoteBoundaryURL]
			if err != nil || !complete(entry) || entry.Dimensions.Width != 20 || entry.Dimensions.Height != 10 || entry.Error != "" || entry.RetryAfter != 0 {
				t.Fatalf("exact limit metadata: %+v %v", entry, err)
			}
			f.assertOriginal(t)
		})
	}
}

type remoteBoundaryWaitingBody struct {
	ctx     context.Context
	started chan struct{}
	closed  bool
}

func (b *remoteBoundaryWaitingBody) Read([]byte) (int, error) {
	close(b.started)
	<-b.ctx.Done()
	return 0, b.ctx.Err()
}

func (b *remoteBoundaryWaitingBody) Close() error {
	b.closed = true
	return nil
}

func TestRemoteRequestUsesDefaultFifteenSecondTimeout(t *testing.T) {
	f := newRemoteBoundaryFixture(t)
	body := &remoteBoundaryWaitingBody{started: make(chan struct{})}
	var deadline time.Time
	var hasDeadline bool
	var received time.Time
	f.processor.Client = &http.Client{Transport: remoteBoundaryTransport(func(r *http.Request) (*http.Response, error) {
		received = time.Now()
		deadline, hasDeadline = r.Context().Deadline()
		body.ctx = r.Context()
		return &http.Response{StatusCode: http.StatusOK, Body: body, ContentLength: -1}, nil
	})}
	started := time.Now()
	// 不给调用方配置较短 deadline：必须实际由正式 fetch 的默认期限结束读取。
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type result struct {
		report Report
		err    error
	}
	done := make(chan result, 1)
	go func() {
		report, err := f.processor.Prepare(ctx, nil, PrepareOptions{Refresh: remoteBoundaryURL})
		done <- result{report, err}
	}()
	var r result
	select {
	case r = <-done:
	case <-time.After(20 * time.Second):
		cancel()
		<-done
		t.Fatal("default request timeout did not end body reading within 20s")
	}
	if !hasDeadline || deadline.Sub(received) < 14*time.Second || deadline.Sub(received) > 15*time.Second {
		t.Fatalf("request did not receive the default 15s deadline: %v %v", hasDeadline, deadline.Sub(received))
	}
	if time.Since(started) < 14*time.Second || !errors.Is(body.ctx.Err(), context.DeadlineExceeded) || !body.closed {
		t.Fatalf("request did not expire while reading body: elapsed=%s ctx=%v closed=%v", time.Since(started), body.ctx.Err(), body.closed)
	}
	f.assertFailure(t, r.report, r.err, "context deadline exceeded")
}

func TestRemoteBodyCancellationRetainsCache(t *testing.T) {
	f := newRemoteBoundaryFixture(t)
	body := &remoteBoundaryWaitingBody{started: make(chan struct{})}
	f.processor.Client = &http.Client{Transport: remoteBoundaryTransport(func(r *http.Request) (*http.Response, error) {
		body.ctx = r.Context()
		return &http.Response{StatusCode: http.StatusOK, Body: body, ContentLength: -1}, nil
	})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type result struct {
		report Report
		err    error
	}
	done := make(chan result, 1)
	go func() {
		report, err := f.processor.Prepare(ctx, nil, PrepareOptions{Refresh: remoteBoundaryURL})
		done <- result{report, err}
	}()
	select {
	case <-body.started:
	case <-time.After(2 * time.Second):
		t.Fatal("remote body read did not start")
	}
	cancel()
	select {
	case r := <-done:
		if !errors.Is(r.err, context.Canceled) || r.report.Processed != 0 || r.report.Failed != 0 || !body.closed {
			t.Fatalf("body cancellation: %+v %v, closed=%v", r.report, r.err, body.closed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("remote body ignored caller cancellation")
	}
	cache, err := os.ReadFile(f.processor.StorePath)
	if err != nil || !bytes.Equal(cache, f.cache) {
		t.Fatalf("canceled remote body changed cache: %v", err)
	}
	f.assertOriginal(t)
}
