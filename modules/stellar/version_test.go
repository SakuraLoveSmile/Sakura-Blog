package stellar

import "testing"

func TestStellarVersion(t *testing.T) {
	if StellarVersion != "1.44.0" {
		t.Fatalf("StellarVersion = %q, want 1.44.0", StellarVersion)
	}
}
