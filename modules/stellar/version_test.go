package stellar

import "testing"

func TestStellarVersion(t *testing.T) {
	if StellarVersion != "2.0.0" {
		t.Fatalf("StellarVersion = %q, want 2.0.0", StellarVersion)
	}
}
