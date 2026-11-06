package ratelimit

import (
	"testing"
	"time"
)

func TestAllowBlocksAfterLimit(t *testing.T) {
	l := New(5, 15*time.Minute)
	for i := 0; i < 5; i++ {
		if !l.Allow("ip") {
			t.Fatalf("attempt %d should allow", i+1)
		}
	}
	if l.Allow("ip") {
		t.Fatal("6th attempt should be blocked")
	}
	if !l.Allow("other") {
		t.Fatal("other key should allow")
	}
}
