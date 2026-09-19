package app

import "testing"

func TestVersionDefaults(t *testing.T) {
	if Version != "dev" {
		t.Fatalf("expected default Version to be 'dev', got %q", Version)
	}
	if GitCommit != "dev" {
		t.Fatalf("expected default GitCommit to be 'dev', got %q", GitCommit)
	}
	if BuildTime != "dev" {
		t.Fatalf("expected default BuildTime to be 'dev', got %q", BuildTime)
	}
	if StellarCommit != "dev" {
		t.Fatalf("expected default StellarCommit to be 'dev', got %q", StellarCommit)
	}
	if FrontendFingerprint != "dev" {
		t.Fatalf("expected default FrontendFingerprint to be 'dev', got %q", FrontendFingerprint)
	}
	if !IsDev() {
		t.Fatal("expected IsDev() to be true for default dev build")
	}
}
