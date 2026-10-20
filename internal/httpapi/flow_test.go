package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/z123-piad/houttuynia-blog/db"
	"github.com/z123-piad/houttuynia-blog/internal/config"
	"github.com/z123-piad/houttuynia-blog/internal/identity"
	"github.com/z123-piad/houttuynia-blog/internal/storage"
)

var (
	testPool *pgxpool.Pool
	testDSN  string
)

func TestMain(m *testing.M) {
	testDSN = os.Getenv("TEST_DATABASE_URL")
	if testDSN == "" {
		testDSN = "postgres://houttuynia:houttuynia@localhost:5432/houttuynia_test?sslmode=disable"
	}
	if !strings.Contains(testDSN, "houttuynia_test") {
		panic("refusing to migrate a database that is not houttuynia_test")
	}
	if err := db.Migrate(testDSN); err != nil {
		panic(err)
	}
	if err := db.Down(testDSN); err != nil {
		panic(err)
	}
	if err := db.Migrate(testDSN); err != nil {
		panic(err)
	}
	pool, err := pgxpool.New(context.Background(), testDSN)
	if err != nil {
		panic(err)
	}
	testPool = pool
	if err := identity.NewService(pool).SeedSuperAdmin(context.Background(), "admin@example.com", "admin123456"); err != nil {
		panic(err)
	}
	code := m.Run()
	pool.Close()
	os.Exit(code)
}

func reset(t *testing.T) {
	t.Helper()
	_, err := testPool.Exec(context.Background(), `
		truncate table sessions, comments, view_daily, post_tags, posts, profiles, assets, friend_links
		restart identity cascade;
		update site_settings set site_name = 'houttuynia', tagline = '' where id = (select id from site_settings order by id limit 1);
	`)
	if err != nil {
		t.Fatal(err)
	}
}

func newAPI(t *testing.T) (*httptest.Server, *storage.Service, string) {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{
		SessionSecret:    "dev-session-secret-change-me",
		StorageDriver:    "local",
		LocalStoragePath: dir,
		PublicBaseURL:    "",
	}
	driver := storage.NewLocal(dir, cfg.SessionSecret, "")
	handler := New(cfg, testPool, driver)
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return ts, storage.NewService(testPool, driver, cfg.SessionSecret), dir
}

type client struct {
	base string
	http *http.Client
}

func newClient(t *testing.T, base string) *client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &client{base: base, http: &http.Client{Jar: jar}}
}

func (c *client) do(t *testing.T, method, path string, body any, dest any) *http.Response {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		rdr = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, c.base+path, rdr)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if dest != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, dest); err != nil {
			t.Fatalf("status %d body %s: %v", res.StatusCode, raw, err)
		}
	}
	res.Body = io.NopCloser(bytes.NewReader(raw))
	return res
}

func (c *client) login(t *testing.T, email, password string) {
	t.Helper()
	res := c.do(t, http.MethodPost, "/api/v1/auth/login", map[string]string{"email": email, "password": password}, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("login %s", res.Status)
	}
}

func TestLoginPublishAndPublicList(t *testing.T) {
	reset(t)
	ts, _, _ := newAPI(t)
	admin := newClient(t, ts.URL)
	admin.login(t, "admin@example.com", "admin123456")

	var category struct {
		ID int64 `json:"id"`
	}
	res := admin.do(t, http.MethodPost, "/api/v1/admin/categories", map[string]any{"name": "笔记", "slug": "notes", "description": "", "sort_order": 1}, &category)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("category %d", res.StatusCode)
	}
	var tag struct {
		ID int64 `json:"id"`
	}
	res = admin.do(t, http.MethodPost, "/api/v1/admin/tags", map[string]any{"name": "Go", "slug": "go"}, &tag)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("tag %d", res.StatusCode)
	}
	var draft struct {
		ID int64 `json:"id"`
	}
	res = admin.do(t, http.MethodPost, "/api/v1/admin/posts", map[string]any{
		"title": "发布说明", "slug": "release-note", "excerpt": "准备发布",
		"body": "正文独有词zzzz", "category_id": category.ID, "tag_ids": []int64{tag.ID},
	}, &draft)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("draft %d", res.StatusCode)
	}
	anon := newClient(t, ts.URL)
	res = anon.do(t, http.MethodGet, "/api/v1/posts/release-note", nil, nil)
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("draft visible %d", res.StatusCode)
	}
	res = admin.do(t, http.MethodPost, "/api/v1/admin/posts/"+itoa(draft.ID)+"/publish", nil, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("publish %d", res.StatusCode)
	}
	var listed []map[string]any
	res = anon.do(t, http.MethodGet, "/api/v1/posts", nil, &listed)
	if res.StatusCode != http.StatusOK || len(listed) != 1 || listed[0]["slug"] != "release-note" {
		t.Fatalf("list %#v %d", listed, res.StatusCode)
	}
	if _, ok := listed[0]["id"]; ok {
		t.Fatal("public list includes post id")
	}
	var found []map[string]any
	res = anon.do(t, http.MethodGet, "/api/v1/search?q="+url.QueryEscape("发布"), nil, &found)
	if res.StatusCode != http.StatusOK || len(found) != 1 {
		t.Fatalf("search title %#v", found)
	}
	var missed []map[string]any
	res = anon.do(t, http.MethodGet, "/api/v1/search?q=zzzz", nil, &missed)
	if res.StatusCode != http.StatusOK || len(missed) != 0 {
		t.Fatalf("search body %#v", missed)
	}
	var months []map[string]any
	res = anon.do(t, http.MethodGet, "/api/v1/archive", nil, &months)
	if res.StatusCode != http.StatusOK || len(months) != 1 {
		t.Fatalf("archive %#v", months)
	}
	var catPage struct {
		Posts []map[string]any `json:"posts"`
	}
	res = anon.do(t, http.MethodGet, "/api/v1/categories/notes", nil, &catPage)
	if res.StatusCode != http.StatusOK || len(catPage.Posts) != 1 {
		t.Fatalf("category page %#v", catPage)
	}
	var tagPage struct {
		Posts []map[string]any `json:"posts"`
	}
	res = anon.do(t, http.MethodGet, "/api/v1/tags/go", nil, &tagPage)
	if res.StatusCode != http.StatusOK || len(tagPage.Posts) != 1 {
		t.Fatalf("tag page %#v", tagPage)
	}
	res = admin.do(t, http.MethodDelete, "/api/v1/admin/posts/"+itoa(draft.ID), nil, nil)
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("delete %d", res.StatusCode)
	}
	res = anon.do(t, http.MethodGet, "/api/v1/posts/release-note", nil, nil)
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("deleted still visible %d", res.StatusCode)
	}
	res = admin.do(t, http.MethodPost, "/api/v1/admin/posts", map[string]any{
		"title": "再写一遍", "slug": "release-note", "excerpt": "", "body": "",
	}, nil)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("reuse slug %d", res.StatusCode)
	}
}

func TestAuthorCannotEditOthersOrPublish(t *testing.T) {
	reset(t)
	ts, _, _ := newAPI(t)
	ctx := context.Background()
	users := identity.NewService(testPool)
	authorID, err := users.EnsureUser(ctx, "author@example.com", "author123456", "作者", "author")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupUser(t, authorID) })
	admin := newClient(t, ts.URL)
	admin.login(t, "admin@example.com", "admin123456")
	var adminPost struct {
		ID int64 `json:"id"`
	}
	res := admin.do(t, http.MethodPost, "/api/v1/admin/posts", map[string]any{
		"title": "管理员草稿", "slug": "admin-draft", "excerpt": "", "body": "x",
	}, &adminPost)
	if res.StatusCode != http.StatusCreated {
		t.Fatal(res.Status)
	}
	author := newClient(t, ts.URL)
	author.login(t, "author@example.com", "author123456")
	var own struct {
		ID int64 `json:"id"`
	}
	res = author.do(t, http.MethodPost, "/api/v1/admin/posts", map[string]any{
		"title": "作者草稿", "slug": "author-draft", "excerpt": "", "body": "y",
	}, &own)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("author create %d", res.StatusCode)
	}
	res = author.do(t, http.MethodPost, "/api/v1/admin/posts/"+itoa(own.ID)+"/publish", nil, nil)
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("author publish %d", res.StatusCode)
	}
	res = author.do(t, http.MethodPut, "/api/v1/admin/posts/"+itoa(adminPost.ID), map[string]any{
		"title": "改别人的", "slug": "admin-draft", "excerpt": "", "body": "no",
	}, nil)
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("author edit other %d", res.StatusCode)
	}
	res = author.do(t, http.MethodPut, "/api/v1/admin/posts/"+itoa(own.ID), map[string]any{
		"title": "改自己的", "slug": "author-draft", "excerpt": "", "body": "yes",
	}, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("author edit own %d", res.StatusCode)
	}
}

func TestAuthzReloadDoesNotLogout(t *testing.T) {
	reset(t)
	ts, _, _ := newAPI(t)
	admin := newClient(t, ts.URL)
	admin.login(t, "admin@example.com", "admin123456")
	res := admin.do(t, http.MethodGet, "/api/v1/auth/me", nil, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatal(res.Status)
	}
	var draft struct {
		ID int64 `json:"id"`
	}
	res = admin.do(t, http.MethodPost, "/api/v1/admin/posts", map[string]any{
		"title": "权限缓存", "slug": "authz-cache", "excerpt": "", "body": "",
	}, &draft)
	if res.StatusCode != http.StatusCreated {
		t.Fatal(res.Status)
	}
	ctx := context.Background()
	if _, err := testPool.Exec(ctx, `
		delete from role_permissions rp
		using roles r, permissions p
		where rp.role_id = r.id and rp.permission_id = p.id
		  and r.name = 'super_admin' and p.code = 'post.publish'
	`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `
			insert into role_permissions (role_id, permission_id, scope)
			select r.id, p.id, 'any'
			from roles r join permissions p on p.code = 'post.publish'
			where r.name = 'super_admin'
			on conflict (role_id, permission_id) do nothing
		`)
	})
	res = admin.do(t, http.MethodPost, "/api/v1/admin/posts/"+itoa(draft.ID)+"/publish", nil, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("cached publish %d", res.StatusCode)
	}
	var roleID int64
	if err := testPool.QueryRow(ctx, `select id from roles where name = 'super_admin'`).Scan(&roleID); err != nil {
		t.Fatal(err)
	}
	if err := identity.NewService(testPool).BumpRole(ctx, roleID); err != nil {
		t.Fatal(err)
	}
	res = admin.do(t, http.MethodPost, "/api/v1/admin/posts/"+itoa(draft.ID)+"/publish", nil, nil)
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("reloaded publish %d", res.StatusCode)
	}
	res = admin.do(t, http.MethodGet, "/api/v1/auth/me", nil, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("still logged in %d", res.StatusCode)
	}
}

func TestLocalUploadAndSignedURL(t *testing.T) {
	reset(t)
	ts, files, _ := newAPI(t)
	admin := newClient(t, ts.URL)
	admin.login(t, "admin@example.com", "admin123456")
	png := tinyPNG()
	var cred struct {
		UploadURL    string            `json:"upload_url"`
		Headers      map[string]string `json:"headers"`
		ObjectKey    string            `json:"object_key"`
		PublicURL    string            `json:"public_url"`
		ConfirmToken string            `json:"confirm_token"`
	}
	res := admin.do(t, http.MethodPost, "/api/v1/assets/upload-token", map[string]any{
		"mime": "image/png", "size": len(png), "visibility": "public",
	}, &cred)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("token %d", res.StatusCode)
	}
	req, err := http.NewRequest(http.MethodPut, ts.URL+cred.UploadURL, bytes.NewReader(png))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Upload-Token", cred.Headers["X-Upload-Token"])
	put, err := admin.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	put.Body.Close()
	if put.StatusCode != http.StatusOK {
		t.Fatalf("put %d", put.StatusCode)
	}
	sum := sha256.Sum256(png)
	var saved struct {
		ID        int64  `json:"id"`
		PublicURL string `json:"public_url"`
	}
	res = admin.do(t, http.MethodPost, "/api/v1/assets", map[string]any{
		"object_key": cred.ObjectKey, "checksum": hex.EncodeToString(sum[:]), "confirm_token": cred.ConfirmToken,
	}, &saved)
	if res.StatusCode != http.StatusCreated || saved.ID == 0 {
		t.Fatalf("confirm %d id %d", res.StatusCode, saved.ID)
	}
	got, err := http.Get(ts.URL + saved.PublicURL)
	if err != nil {
		t.Fatal(err)
	}
	got.Body.Close()
	if got.StatusCode != http.StatusOK {
		t.Fatalf("public media %d", got.StatusCode)
	}
	actor, err := identity.NewService(testPool).LookupSession(context.Background(), sessionToken(t, admin, ts.URL))
	if err != nil {
		t.Fatal(err)
	}
	signed, err := files.SignedURL(context.Background(), actor, saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	signedRes, err := http.Get(ts.URL + signed)
	if err != nil {
		t.Fatal(err)
	}
	signedRes.Body.Close()
	if signedRes.StatusCode != http.StatusOK {
		t.Fatalf("signed %d %s", signedRes.StatusCode, signed)
	}
	bare, err := http.Get(ts.URL + "/media-signed/" + cred.ObjectKey)
	if err != nil {
		t.Fatal(err)
	}
	bare.Body.Close()
	if bare.StatusCode == http.StatusOK {
		t.Fatal("unsigned media-signed returned 200")
	}
	res = admin.do(t, http.MethodPost, "/api/v1/assets/"+itoa(saved.ID)+"/signed-url", nil, nil)
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("signed-url route %d", res.StatusCode)
	}
}

func TestReservedRoutesAndConstraints(t *testing.T) {
	reset(t)
	ts, _, _ := newAPI(t)
	anon := newClient(t, ts.URL)
	for _, path := range []string{"/api/v1/comments", "/api/v1/friend-links", "/api/v1/stats", "/api/v1/oauth/wechat"} {
		res := anon.do(t, http.MethodGet, path, nil, nil)
		if res.StatusCode != http.StatusNotFound {
			t.Fatalf("%s %d", path, res.StatusCode)
		}
	}
	ctx := context.Background()
	var adminID int64
	if err := testPool.QueryRow(ctx, `select id from users where email = 'admin@example.com'`).Scan(&adminID); err != nil {
		t.Fatal(err)
	}
	_, err := testPool.Exec(ctx, `
		insert into user_identities (user_id, provider, app_id, provider_user_id, raw)
		values ($1, 'wechat', 'app', 'openid-only', '{}'::jsonb)
	`, adminID)
	if err == nil {
		t.Fatal("wechat without union_id was accepted")
	}
	_, err = testPool.Exec(ctx, `delete from roles where name = 'author'`)
	if err == nil {
		t.Fatal("builtin role delete was accepted")
	}
}

func sessionToken(t *testing.T, c *client, base string) string {
	t.Helper()
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	for _, cookie := range c.http.Jar.Cookies(u) {
		if cookie.Name == "session" {
			return cookie.Value
		}
	}
	t.Fatal("missing session cookie")
	return ""
}

func cleanupUser(t *testing.T, userID int64) {
	t.Helper()
	ctx := context.Background()
	statements := []string{
		`delete from post_tags where post_id in (select id from posts where author_id = $1)`,
		`delete from posts where author_id = $1`,
		`delete from sessions where user_id = $1`,
		`delete from assets where owner_id = $1`,
		`delete from user_identities where user_id = $1`,
		`delete from user_roles where user_id = $1`,
		`delete from users where id = $1`,
	}
	for _, statement := range statements {
		if _, err := testPool.Exec(ctx, statement, userID); err != nil {
			t.Fatal(err)
		}
	}
}

func itoa(n int64) string {
	return strconvFormat(n)
}

func strconvFormat(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func tinyPNG() []byte {
	return []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89,
		0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41, 0x54,
		0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00, 0x05, 0x00, 0x01,
		0x0D, 0x0A, 0x2D, 0xB4,
		0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
	}
}
