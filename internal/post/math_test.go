package post

import (
	"strings"
	"testing"
)

func TestProtectMathDelimiters(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
		not  []string
	}{
		{
			name: "inline dollar unchanged",
			in:   "Energy $E=mc^2$ here",
			want: []string{"$E=mc^2$"},
			not:  []string{"<span>", "<div>"},
		},
		{
			name: "block dollar wrapped",
			in:   "Before\n\n$$\\int_0^1 x\\,dx$$\n\nAfter",
			want: []string{"<div>$$", "\\,dx$$</div>"},
		},
		{
			name: "paren encoded",
			in:   "Alpha \\(\\alpha + \\beta\\) beta",
			want: []string{"<span>", "&#92;(", "&#92;)", "</span>"},
		},
		{
			name: "bracket encoded",
			in:   "Sum \\[\\sum_{i=1}^n i\\]",
			want: []string{"<div>", "&#92;[", "&#92;]", "</div>"},
		},
		{
			name: "price text untouched",
			in:   "Price $5 and $10",
			want: []string{"$5 and $10"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := protectMathDelimiters(tt.in)
			for _, sub := range tt.want {
				if !strings.Contains(got, sub) {
					t.Fatalf("protectMathDelimiters(%q) = %q, want substring %q", tt.in, got, sub)
				}
			}
			for _, sub := range tt.not {
				if strings.Contains(got, sub) {
					t.Fatalf("protectMathDelimiters(%q) = %q, must not contain %q", tt.in, got, sub)
				}
			}
		})
	}
}
