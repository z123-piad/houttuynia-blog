package stats

import "testing"

func TestVisitorHash(t *testing.T) {
	if _, err := VisitorHash("", "1.2.3.4", "ua"); err == nil {
		t.Fatal("expected empty key error")
	}
	a, err := VisitorHash("secret", "1.2.3.4", "ua")
	if err != nil {
		t.Fatal(err)
	}
	b, err := VisitorHash("secret", "1.2.3.4", "ua")
	if err != nil {
		t.Fatal(err)
	}
	if a != b || len(a) != 64 {
		t.Fatalf("hash = %q", a)
	}
	c, err := VisitorHash("secret", "9.9.9.9", "ua")
	if err != nil {
		t.Fatal(err)
	}
	if a == c {
		t.Fatal("expected different inputs to differ")
	}
}
