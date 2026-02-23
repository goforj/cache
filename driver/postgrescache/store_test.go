package postgrescache

import "testing"

func TestNewRequiresDSN(t *testing.T) {
	store, err := New(Config{})
	if err == nil {
		t.Fatalf("expected error for missing dsn")
	}
	if store != nil {
		t.Fatalf("expected nil store on error")
	}
}
