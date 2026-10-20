package stats

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

// VisitorHash is the reserved HMAC-SHA256 of IP and User-Agent.
// v1 does not call it from HTTP and does not increment view_count.
func VisitorHash(key, ip, userAgent string) (string, error) {
	if key == "" {
		return "", errors.New("VISITOR_HASH_KEY is empty")
	}
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte(ip))
	_, _ = mac.Write([]byte{'\n'})
	_, _ = mac.Write([]byte(userAgent))
	return hex.EncodeToString(mac.Sum(nil)), nil
}

// Repository is reserved. v1 does not write view_daily.
type Repository interface {
	// Record is intentionally unimplemented in v1.
}
