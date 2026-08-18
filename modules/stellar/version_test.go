package stellar

import "testing"

func TestStellarVersion(t *testing.T) {
	if StellarVersion != "1.42.1" {
		t.Fatalf("StellarVersion = %q, want 1.42.1", StellarVersion)
	}
}
