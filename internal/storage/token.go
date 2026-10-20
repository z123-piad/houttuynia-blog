package storage

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type tokenPayload struct {
	Key        string `json:"k"`
	Mime       string `json:"m"`
	Size       int64  `json:"z"`
	Visibility string `json:"v"`
	OwnerID    int64  `json:"o"`
	Driver     string `json:"d"`
	Exp        int64  `json:"e"`
	Kind       string `json:"t"`
}

func signToken(secret string, payload tokenPayload) (string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	body := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(body))
	return body + "." + hex.EncodeToString(mac.Sum(nil)), nil
}

func openToken(secret, token, kind string) (tokenPayload, error) {
	body, sig, ok := strings.Cut(token, ".")
	if !ok || body == "" || sig == "" {
		return tokenPayload{}, errors.New("bad token")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(body))
	want := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(want)) {
		return tokenPayload{}, errors.New("bad token")
	}
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return tokenPayload{}, err
	}
	var payload tokenPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return tokenPayload{}, err
	}
	if payload.Kind != kind || payload.Exp < time.Now().Unix() {
		return tokenPayload{}, errors.New("expired token")
	}
	return payload, nil
}

func escapeKey(key string) string {
	parts := strings.Split(key, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func signQuery(secret, key string, exp int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(key))
	_, _ = mac.Write([]byte{'\n'})
	_, _ = mac.Write([]byte(strconv.FormatInt(exp, 10)))
	return hex.EncodeToString(mac.Sum(nil))
}

func verifyQuery(secret, key, sig string, exp int64) bool {
	if exp < time.Now().Unix() {
		return false
	}
	want := signQuery(secret, key, exp)
	return hmac.Equal([]byte(sig), []byte(want))
}
