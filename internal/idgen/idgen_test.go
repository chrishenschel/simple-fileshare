package idgen

import (
	"regexp"
	"testing"
)

func TestNewLengthAndCharset(t *testing.T) {
	re := regexp.MustCompile(`^[A-Za-z0-9]{5}$`)
	for i := 0; i < 50; i++ {
		id, err := New(5)
		if err != nil {
			t.Fatal(err)
		}
		if !re.MatchString(id) {
			t.Fatalf("bad id %q", id)
		}
	}
}
