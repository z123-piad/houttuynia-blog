package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/z123-piad/houttuynia-blog/internal/app"
	"github.com/z123-piad/houttuynia-blog/internal/content"
	"github.com/z123-piad/houttuynia-blog/internal/storage"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	var ae *app.Error
	if errors.As(err, &ae) {
		writeJSON(w, ae.Status, map[string]string{"error": ae.Message})
		return
	}
	logServer(err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "服务器错误"})
}

func errForbidden() error { return app.Forbidden("没有权限") }

func decode(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return app.Invalid("请求格式不正确")
	}
	return nil
}

func limitOffset(r *http.Request, def, max int32) (int32, int32) {
	lim := int32(atoiDefault(r.URL.Query().Get("limit"), int(def)))
	off := int32(atoiDefault(r.URL.Query().Get("offset"), 0))
	if lim < 1 {
		lim = def
	}
	if lim > max {
		lim = max
	}
	if off < 0 {
		off = 0
	}
	return lim, off
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

func (s *Server) getSite(w http.ResponseWriter, r *http.Request) {
	settings, err := s.site.Get(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) listPosts(w http.ResponseWriter, r *http.Request) {
	lim, off := limitOffset(r, 20, 100)
	posts, err := s.content.ListPublished(r.Context(), lim, off)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, orEmpty(posts))
}

func (s *Server) getPost(w http.ResponseWriter, r *http.Request) {
	post, err := s.content.GetPublished(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, post)
}

func (s *Server) listCategories(w http.ResponseWriter, r *http.Request) {
	rows, err := s.content.ListCategories(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, orEmpty(rows))
}

func (s *Server) categoryPosts(w http.ResponseWriter, r *http.Request) {
	cat, posts, err := s.content.ListByCategory(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"category": cat, "posts": orEmpty(posts)})
}

func (s *Server) listTags(w http.ResponseWriter, r *http.Request) {
	rows, err := s.content.ListTags(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, orEmpty(rows))
}

func (s *Server) tagPosts(w http.ResponseWriter, r *http.Request) {
	tag, posts, err := s.content.ListByTag(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tag": tag, "posts": orEmpty(posts)})
}

func (s *Server) archive(w http.ResponseWriter, r *http.Request) {
	months, err := s.content.Archive(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	if months == nil {
		months = []content.ArchiveMonth{}
	}
	writeJSON(w, http.StatusOK, months)
}

func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	posts, err := s.content.Search(r.Context(), r.URL.Query().Get("q"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, orEmpty(posts))
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decode(w, r, &body); err != nil {
		writeErr(w, err)
		return
	}
	token, expires, err := s.identity.Login(r.Context(), body.Email, body.Password)
	if err != nil {
		writeErr(w, err)
		return
	}
	s.setSession(w, token, expires)
	writeJSON(w, http.StatusOK, map[string]string{"email": strings.ToLower(strings.TrimSpace(body.Email))})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	token := ""
	if cookie, err := r.Cookie("session"); err == nil {
		token = cookie.Value
	}
	if err := s.identity.Logout(r.Context(), token); err != nil {
		writeErr(w, err)
		return
	}
	s.clearSession(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	p := current(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"id": p.UserID, "email": p.Email, "display_name": p.DisplayName, "roles": p.Roles,
	})
}

func (s *Server) menus(w http.ResponseWriter, r *http.Request) {
	rows, err := s.identity.Menus(r.Context(), current(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	out := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]string{"name": row.Name, "route": row.Route})
	}
	writeJSON(w, http.StatusOK, out)
}

type postBody struct {
	Title      string  `json:"title"`
	Slug       string  `json:"slug"`
	Excerpt    string  `json:"excerpt"`
	Body       string  `json:"body"`
	CategoryID *int64  `json:"category_id"`
	TagIDs     []int64 `json:"tag_ids"`
}

func (b postBody) input() content.PostInput {
	return content.PostInput{
		Title: b.Title, Slug: b.Slug, Excerpt: b.Excerpt, Body: b.Body,
		CategoryID: b.CategoryID, TagIDs: b.TagIDs,
	}
}

func (s *Server) adminListPosts(w http.ResponseWriter, r *http.Request) {
	lim, off := limitOffset(r, 50, 100)
	posts, err := s.content.ListAdmin(r.Context(), current(r), lim, off)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, orEmpty(posts))
}

func (s *Server) adminCreatePost(w http.ResponseWriter, r *http.Request) {
	var body postBody
	if err := decode(w, r, &body); err != nil {
		writeErr(w, err)
		return
	}
	post, err := s.content.Create(r.Context(), current(r), body.input())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, post)
}

func (s *Server) adminGetPost(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	post, err := s.content.GetAdmin(r.Context(), current(r), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, post)
}

func (s *Server) adminUpdatePost(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	var body postBody
	if err := decode(w, r, &body); err != nil {
		writeErr(w, err)
		return
	}
	post, err := s.content.Update(r.Context(), current(r), id, body.input())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, post)
}

func (s *Server) adminDeletePost(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	if err := s.content.SoftDelete(r.Context(), current(r), id); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) adminPublishPost(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	post, err := s.content.Publish(r.Context(), current(r), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, post)
}

type nameBody struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	SortOrder   int32  `json:"sort_order"`
}

func (s *Server) adminCreateCategory(w http.ResponseWriter, r *http.Request) {
	var body nameBody
	if err := decode(w, r, &body); err != nil {
		writeErr(w, err)
		return
	}
	row, err := s.content.CreateCategory(r.Context(), body.Name, body.Slug, body.Description, body.SortOrder)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, row)
}

func (s *Server) adminUpdateCategory(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	var body nameBody
	if err := decode(w, r, &body); err != nil {
		writeErr(w, err)
		return
	}
	row, err := s.content.UpdateCategory(r.Context(), id, body.Name, body.Slug, body.Description, body.SortOrder)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (s *Server) adminDeleteCategory(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	if err := s.content.DeleteCategory(r.Context(), id); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) adminCreateTag(w http.ResponseWriter, r *http.Request) {
	var body nameBody
	if err := decode(w, r, &body); err != nil {
		writeErr(w, err)
		return
	}
	row, err := s.content.CreateTag(r.Context(), body.Name, body.Slug)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, row)
}

func (s *Server) adminUpdateTag(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	var body nameBody
	if err := decode(w, r, &body); err != nil {
		writeErr(w, err)
		return
	}
	row, err := s.content.UpdateTag(r.Context(), id, body.Name, body.Slug)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (s *Server) adminDeleteTag(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	if err := s.content.DeleteTag(r.Context(), id); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) adminUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SiteName string `json:"site_name"`
		Tagline  string `json:"tagline"`
	}
	if err := decode(w, r, &body); err != nil {
		writeErr(w, err)
		return
	}
	row, err := s.site.Update(r.Context(), body.SiteName, body.Tagline)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (s *Server) uploadToken(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Mime       string `json:"mime"`
		Size       int64  `json:"size"`
		Visibility string `json:"visibility"`
	}
	if err := decode(w, r, &body); err != nil {
		writeErr(w, err)
		return
	}
	cred, err := s.storage.IssueCredential(r.Context(), current(r), body.Mime, body.Size, body.Visibility)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cred)
}

func (s *Server) confirmAsset(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ObjectKey    string `json:"object_key"`
		Checksum     string `json:"checksum"`
		ConfirmToken string `json:"confirm_token"`
	}
	if err := decode(w, r, &body); err != nil {
		writeErr(w, err)
		return
	}
	cred, err := s.storage.Confirm(r.Context(), current(r), storage.ConfirmInput{
		ObjectKey: body.ObjectKey, Checksum: body.Checksum, ConfirmToken: body.ConfirmToken,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, cred)
}

func (s *Server) deleteAsset(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	if err := s.storage.Delete(r.Context(), current(r), id); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) localUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<20+1)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeErr(w, app.Invalid("文件过大或读取失败"))
		return
	}
	if err := s.storage.AcceptLocalUpload(r.Header.Get("X-Upload-Token"), body); err != nil {
		writeErr(w, err)
		return
	}
	sum := sha256.Sum256(body)
	writeJSON(w, http.StatusOK, map[string]string{"checksum": hex.EncodeToString(sum[:])})
}

func (s *Server) publicMedia(w http.ResponseWriter, r *http.Request) {
	key, err := wildcardKey(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	mime, file, err := s.storage.OpenPublic(r.Context(), key)
	if err != nil {
		writeErr(w, err)
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = io.Copy(w, file)
}

func (s *Server) signedMedia(w http.ResponseWriter, r *http.Request) {
	key, err := wildcardKey(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	exp, _ := strconv.ParseInt(r.URL.Query().Get("exp"), 10, 64)
	mime, file, err := s.storage.OpenSigned(r.Context(), key, r.URL.Query().Get("sig"), exp)
	if err != nil {
		writeErr(w, err)
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "private, max-age=0")
	_, _ = io.Copy(w, file)
}

func wildcardKey(r *http.Request) (string, error) {
	raw := strings.TrimPrefix(chi.URLParam(r, "*"), "/")
	key, err := url.PathUnescape(raw)
	if err != nil || key == "" {
		return "", app.NotFound("文件不存在")
	}
	return key, nil
}

func pathID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		return 0, app.NotFound("不存在")
	}
	return id, nil
}

func orEmpty[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}
