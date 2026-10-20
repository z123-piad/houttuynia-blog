package identity

import "testing"

func TestPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("admin123456")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(hash, "admin123456") {
		t.Fatal("expected password to match")
	}
	if VerifyPassword(hash, "wrong-password") {
		t.Fatal("expected mismatch")
	}
}
