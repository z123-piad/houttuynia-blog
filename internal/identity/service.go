package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/z123-piad/houttuynia-blog/internal/app"
	"github.com/z123-piad/houttuynia-blog/internal/sqlc"
)

const SessionTTL = 14 * 24 * time.Hour

type Principal struct {
	SessionID   int64
	UserID      int64
	Email       string
	DisplayName string
	Roles       []string
	Grants      map[string]string
}

func (p Principal) Has(code string) bool {
	_, ok := p.Grants[code]
	return ok
}

// Allows reports the winning scope. "any" wins over "own".
func (p Principal) Allows(code string) (string, bool) {
	scope, ok := p.Grants[code]
	return scope, ok
}

func (p Principal) Can(code string, ownerID int64) bool {
	switch p.Grants[code] {
	case "any":
		return true
	case "own":
		return p.UserID == ownerID
	default:
		return false
	}
}

type Service struct {
	q     *sqlc.Queries
	pool  *pgxpool.Pool
	mu    sync.Mutex
	cache map[int64]cachedGrant
}

type cachedGrant struct {
	Version int32
	Roles   []string
	Grants  map[string]string
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{q: sqlc.New(pool), pool: pool, cache: map[int64]cachedGrant{}}
}

func (s *Service) SeedSuperAdmin(ctx context.Context, email, password string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || password == "" {
		return nil
	}
	if len(password) < 8 {
		return app.Invalid("管理员密码至少 8 位")
	}
	_, err := s.EnsureUser(ctx, email, password, "超级管理员", "super_admin")
	return err
}

// EnsureUser creates a user when the email is new. An existing email keeps its password.
func (s *Service) EnsureUser(ctx context.Context, email, password, displayName, roleName string) (int64, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	displayName = strings.TrimSpace(displayName)
	if email == "" || displayName == "" || roleName == "" {
		return 0, app.Invalid("用户信息不完整")
	}
	role, err := s.q.GetRoleByName(ctx, roleName)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, app.NotFound("角色不存在")
	}
	if err != nil {
		return 0, err
	}
	user, err := s.q.GetUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		if len(password) < 8 {
			return 0, app.Invalid("密码至少 8 位")
		}
		hash, hashErr := HashPassword(password)
		if hashErr != nil {
			return 0, hashErr
		}
		created, createErr := s.q.CreateUser(ctx, sqlc.CreateUserParams{
			Email:        email,
			PasswordHash: &hash,
			DisplayName:  displayName,
		})
		if createErr != nil {
			if app.UniqueViolation(createErr) {
				return 0, app.Conflict("邮箱已被使用")
			}
			return 0, createErr
		}
		userID := created.ID
		if err := s.attach(ctx, userID, role.ID, email); err != nil {
			return 0, err
		}
		return userID, nil
	}
	if err != nil {
		return 0, err
	}
	if err := s.q.AssignUserRole(ctx, sqlc.AssignUserRoleParams{UserID: user.ID, RoleID: role.ID}); err != nil {
		return 0, err
	}
	has, err := s.q.HasPasswordIdentity(ctx, user.ID)
	if err != nil {
		return 0, err
	}
	if !has {
		if err := s.q.CreateUserIdentity(ctx, sqlc.CreateUserIdentityParams{
			UserID:         user.ID,
			Provider:       "password",
			AppID:          "local",
			ProviderUserID: email,
			Raw:            []byte("{}"),
		}); err != nil && !app.UniqueViolation(err) {
			return 0, err
		}
	}
	return user.ID, nil
}

func (s *Service) attach(ctx context.Context, userID, roleID int64, email string) error {
	if err := s.q.AssignUserRole(ctx, sqlc.AssignUserRoleParams{UserID: userID, RoleID: roleID}); err != nil {
		return err
	}
	return s.q.CreateUserIdentity(ctx, sqlc.CreateUserIdentityParams{
		UserID:         userID,
		Provider:       "password",
		AppID:          "local",
		ProviderUserID: email,
		Raw:            []byte("{}"),
	})
}

func (s *Service) Login(ctx context.Context, email, password string) (token string, expires time.Time, err error) {
	email = strings.ToLower(strings.TrimSpace(email))
	user, err := s.q.GetUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (user.Status != "active" || user.PasswordHash == nil || !VerifyPassword(*user.PasswordHash, password))) {
		return "", time.Time{}, app.Unauthorized("邮箱或密码不正确")
	}
	if err != nil {
		return "", time.Time{}, err
	}
	has, err := s.q.HasPasswordIdentity(ctx, user.ID)
	if err != nil {
		return "", time.Time{}, err
	}
	if !has {
		return "", time.Time{}, app.Unauthorized("登录身份不完整")
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", time.Time{}, err
	}
	token = hex.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token))
	expires = time.Now().Add(SessionTTL)
	if _, err := s.q.CreateSession(ctx, sqlc.CreateSessionParams{
		UserID:       user.ID,
		TokenHash:    hex.EncodeToString(sum[:]),
		ExpiresAt:    expires,
		AuthzVersion: user.AuthzVersion,
	}); err != nil {
		return "", time.Time{}, err
	}
	return token, expires, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	sum := sha256.Sum256([]byte(token))
	row, err := s.q.GetSessionByTokenHash(ctx, hex.EncodeToString(sum[:]))
	if err == nil {
		s.mu.Lock()
		delete(s.cache, row.ID)
		s.mu.Unlock()
	}
	return s.q.DeleteSessionByTokenHash(ctx, hex.EncodeToString(sum[:]))
}

func (s *Service) LookupSession(ctx context.Context, token string) (Principal, error) {
	if token == "" {
		return Principal{}, app.Unauthorized("请先登录")
	}
	sum := sha256.Sum256([]byte(token))
	row, err := s.q.GetSessionByTokenHash(ctx, hex.EncodeToString(sum[:]))
	if errors.Is(err, pgx.ErrNoRows) {
		return Principal{}, app.Unauthorized("请先登录")
	}
	if err != nil {
		return Principal{}, err
	}
	if row.DeletedAt != nil || row.Status != "active" || time.Now().After(row.ExpiresAt) {
		return Principal{}, app.Unauthorized("请先登录")
	}
	if row.UserAuthzVersion == row.AuthzVersion {
		s.mu.Lock()
		cached, ok := s.cache[row.ID]
		s.mu.Unlock()
		if ok && cached.Version == row.UserAuthzVersion {
			return Principal{
				SessionID:   row.ID,
				UserID:      row.UserID,
				Email:       row.Email,
				DisplayName: row.DisplayName,
				Roles:       cached.Roles,
				Grants:      cached.Grants,
			}, nil
		}
	}
	roles, grants, err := s.loadGrants(ctx, row.UserID)
	if err != nil {
		return Principal{}, err
	}
	if row.UserAuthzVersion != row.AuthzVersion {
		if err := s.q.UpdateSessionAuthzVersion(ctx, sqlc.UpdateSessionAuthzVersionParams{
			ID:           row.ID,
			AuthzVersion: row.UserAuthzVersion,
		}); err != nil {
			return Principal{}, err
		}
	}
	s.mu.Lock()
	s.cache[row.ID] = cachedGrant{Version: row.UserAuthzVersion, Roles: roles, Grants: grants}
	s.mu.Unlock()
	return Principal{
		SessionID:   row.ID,
		UserID:      row.UserID,
		Email:       row.Email,
		DisplayName: row.DisplayName,
		Roles:       roles,
		Grants:      grants,
	}, nil
}

func (s *Service) loadGrants(ctx context.Context, userID int64) ([]string, map[string]string, error) {
	roles, err := s.q.ListUserRoleNames(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	rows, err := s.q.ListUserPermissionScopes(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	grants := map[string]string{}
	for _, row := range rows {
		if grants[row.Code] == "any" {
			continue
		}
		grants[row.Code] = row.Scope
	}
	return roles, grants, nil
}

func (s *Service) BumpRole(ctx context.Context, roleID int64) error {
	return s.q.BumpAuthzVersionForRole(ctx, roleID)
}

func (s *Service) Menus(ctx context.Context, p Principal) ([]sqlc.ListMenusRow, error) {
	rows, err := s.q.ListMenus(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]sqlc.ListMenusRow, 0, len(rows))
	for _, row := range rows {
		if p.Has(row.PermissionCode) {
			out = append(out, row)
		}
	}
	return out, nil
}
