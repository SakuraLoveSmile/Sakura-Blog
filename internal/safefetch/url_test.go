package safefetch

import (
	"context"
	"errors"
	"net"
	"testing"
)

func TestValidateURL(t *testing.T) {
	for _, raw := range []string{"file:///etc/passwd", "ftp://example.com/a", "https://user@example.com/a", "http://127.0.0.1/a", "http://192.168.1.1/a"} {
		_, err := ValidateURL(raw)
		if err == nil {
			t.Errorf("ValidateURL(%q) unexpectedly succeeded", raw)
		}
	}
	if _, err := ValidateURL("https://example.com/image.png"); err != nil {
		t.Fatalf("public HTTPS URL rejected: %v", err)
	}
	if _, err := ValidateURL("http://127.0.0.1/a"); !errors.Is(err, ErrForbiddenHost) {
		t.Fatalf("loopback error = %v, want ErrForbiddenHost", err)
	}
}

func TestClientRejectsPrivateDNSResolutionBeforeDial(t *testing.T) {
	dialCalled := false
	client := New(Options{
		ResolveIP: func(context.Context, string) ([]net.IP, error) {
			return []net.IP{net.ParseIP("10.0.0.1")}, nil
		},
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			dialCalled = true
			return nil, errors.New("unexpected dial")
		},
	})
	_, err := client.Get("http://rebind.test:80/image.png")
	if !errors.Is(err, ErrForbiddenHost) {
		t.Fatalf("error = %v, want ErrForbiddenHost", err)
	}
	if dialCalled {
		t.Fatal("forbidden DNS result should be rejected before DialContext")
	}
}
