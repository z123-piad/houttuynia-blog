package identity

import (
	"context"
	"errors"
)

// Identity is the result of exchanging an authorization code.
// Wechat implementations must set UnionID.
type Identity struct {
	ProviderUserID string
	UnionID        string
	AccessToken    string
	RefreshToken   string
	Raw            []byte
}

// Provider is the reserved third-party login interface. HTTP routes are not registered.
type Provider interface {
	AuthorizationURL(state string) (string, error)
	Exchange(ctx context.Context, code string) (Identity, error)
}

// UnconfiguredProvider refuses every call. It does not pretend a login succeeded.
type UnconfiguredProvider struct {
	Name string
}

func (p UnconfiguredProvider) AuthorizationURL(string) (string, error) {
	return "", errors.New(p.Name + " provider is not configured")
}

func (p UnconfiguredProvider) Exchange(context.Context, string) (Identity, error) {
	return Identity{}, errors.New(p.Name + " provider is not configured")
}

// RequireWechatUnionID rejects a wechat identity that has no union_id.
func RequireWechatUnionID(provider string, unionID string) error {
	if provider == "wechat" && len(stringsTrim(unionID)) == 0 {
		return errors.New("wechat identity requires union_id")
	}
	return nil
}

func stringsTrim(s string) string {
	i := 0
	j := len(s)
	for i < j && (s[i] == ' ' || s[i] == '\n' || s[i] == '\t') {
		i++
	}
	for j > i && (s[j-1] == ' ' || s[j-1] == '\n' || s[j-1] == '\t') {
		j--
	}
	return s[i:j]
}

// Profile is reserved. The about page does not read it, and no HTTP route edits it.
type Profile struct {
	UserID        int64
	Bio           string
	AvatarAssetID *int64
	SocialLinks   []byte
}
