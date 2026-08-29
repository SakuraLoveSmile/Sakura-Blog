package settings

import "testing"

func TestCardHoverConfigDefaultsAndBounds(t *testing.T) {
	var empty PluginsConfig
	if empty.CardHoverEnabled() {
		t.Fatal("card_hover should be opt-in")
	}
	if got := empty.CardHoverMaxTilt(); got != 3 {
		t.Fatalf("empty max tilt = %v, want 3", got)
	}

	for _, tc := range []struct {
		name string
		max  float64
		want float64
	}{
		{name: "zero", max: 0, want: 0},
		{name: "fraction", max: 4.5, want: 4.5},
		{name: "negative", max: -1, want: 3},
		{name: "too large", max: 9, want: 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := PluginsConfig{CardHover: &CardHoverConfig{Enable: true, MaxTilt: tc.max}}
			if !cfg.CardHoverEnabled() {
				t.Fatal("explicit enable should be true")
			}
			if got := cfg.CardHoverMaxTilt(); got != tc.want {
				t.Fatalf("max tilt = %v, want %v", got, tc.want)
			}
		})
	}
}
