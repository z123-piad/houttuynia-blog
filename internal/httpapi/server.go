package httpapi

import (
	"context"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/z123-piad/houttuynia-blog/internal/config"
	"github.com/z123-piad/houttuynia-blog/internal/content"
	"github.com/z123-piad/houttuynia-blog/internal/identity"
	"github.com/z123-piad/houttuynia-blog/internal/site"
	"github.com/z123-piad/houttuynia-blog/internal/storage"
)

type Server struct {
	cfg      config.Config
	identity *identity.Service
	content  *content.Service
	site     *site.Service
	storage  *storage.Service
}

type principalKey struct{}

func New(cfg config.Config, pool *pgxpool.Pool, driver storage.Driver) http.Handler {
	s := &Server{
		cfg:      cfg,
		identity: identity.NewService(pool),
		content:  content.NewService(pool),
		site:     site.NewService(pool),
		storage:  storage.NewService(pool, driver, cfg.SessionSecret),
	}
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Get("/media/*", s.publicMedia)
	r.Get("/media-signed/*", s.signedMedia)

	r.Route("/api/v1", func(api chi.Router) {
		api.Get("/site", s.getSite)
		api.Get("/posts", s.listPosts)
		api.Get("/posts/{slug}", s.getPost)
		api.Get("/categories", s.listCategories)
		api.Get("/categories/{slug}", s.categoryPosts)
		api.Get("/tags", s.listTags)
		api.Get("/tags/{slug}", s.tagPosts)
		api.Get("/archive", s.archive)
		api.Get("/search", s.search)
		api.Post("/auth/login", s.login)
		api.Put("/storage/local/upload", s.localUpload)

		api.Group(func(priv chi.Router) {
			priv.Use(s.requireAuth)
			priv.Post("/auth/logout", s.logout)
			priv.Get("/auth/me", s.me)
			priv.Get("/admin/menus", s.menus)

			priv.With(s.requirePerm("post.read")).Get("/admin/posts", s.adminListPosts)
			priv.With(s.requirePerm("post.create")).Post("/admin/posts", s.adminCreatePost)
			priv.With(s.requirePerm("post.read")).Get("/admin/posts/{id}", s.adminGetPost)
			priv.With(s.requirePerm("post.update")).Put("/admin/posts/{id}", s.adminUpdatePost)
			priv.With(s.requirePerm("post.delete")).Delete("/admin/posts/{id}", s.adminDeletePost)
			priv.With(s.requirePerm("post.publish")).Post("/admin/posts/{id}/publish", s.adminPublishPost)

			priv.With(s.requirePerm("category.manage")).Post("/admin/categories", s.adminCreateCategory)
			priv.With(s.requirePerm("category.manage")).Put("/admin/categories/{id}", s.adminUpdateCategory)
			priv.With(s.requirePerm("category.manage")).Delete("/admin/categories/{id}", s.adminDeleteCategory)
			priv.With(s.requirePerm("tag.manage")).Post("/admin/tags", s.adminCreateTag)
			priv.With(s.requirePerm("tag.manage")).Put("/admin/tags/{id}", s.adminUpdateTag)
			priv.With(s.requirePerm("tag.manage")).Delete("/admin/tags/{id}", s.adminDeleteTag)

			priv.With(s.requirePerm("setting.manage")).Put("/admin/settings", s.adminUpdateSettings)
			priv.With(s.requirePerm("asset.upload")).Post("/assets/upload-token", s.uploadToken)
			priv.With(s.requirePerm("asset.upload")).Post("/assets", s.confirmAsset)
			priv.With(s.requirePerm("asset.delete")).Delete("/assets/{id}", s.deleteAsset)
		})
	})
	// Issuing a private signed URL stays on storage.Service.SignedURL.
	// The HTTP route is intentionally not registered.

	if cfg.WebDir != "" {
		r.NotFound(s.spa)
	}
	return r
}

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session")
		token := ""
		if err == nil {
			token = cookie.Value
		}
		p, err := s.identity.LookupSession(r.Context(), token)
		if err != nil {
			writeErr(w, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, p)))
	})
}

func (s *Server) requirePerm(code string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !current(r).Has(code) {
				writeErr(w, errForbidden())
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func current(r *http.Request) identity.Principal {
	p, _ := r.Context().Value(principalKey{}).(identity.Principal)
	return p
}

func (s *Server) spa(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/media") {
		http.NotFound(w, r)
		return
	}
	rel := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	root := filepath.Clean(s.cfg.WebDir)
	target := filepath.Join(root, rel)
	if target != root && !strings.HasPrefix(target, root+string(filepath.Separator)) {
		http.NotFound(w, r)
		return
	}
	if info, err := os.Stat(target); err == nil && !info.IsDir() {
		http.ServeFile(w, r, target)
		return
	}
	http.ServeFile(w, r, filepath.Join(root, "index.html"))
}

func (s *Server) setSession(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.cfg.CookieSecure,
		Expires:  expires,
	})
}

func (s *Server) clearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.cfg.CookieSecure,
		MaxAge:   -1,
	})
}

func logServer(err error) {
	if err != nil {
		log.Printf("request error: %v", err)
	}
}
