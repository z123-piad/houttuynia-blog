package site

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/z123-piad/houttuynia-blog/internal/app"
	"github.com/z123-piad/houttuynia-blog/internal/sqlc"
)

type Settings struct {
	SiteName string `json:"site_name"`
	Tagline  string `json:"tagline"`
}

type Service struct {
	q *sqlc.Queries
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{q: sqlc.New(pool)}
}

func (s *Service) Get(ctx context.Context) (Settings, error) {
	row, err := s.q.GetSiteSettings(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return Settings{}, app.NotFound("站点设置不存在")
	}
	if err != nil {
		return Settings{}, err
	}
	return Settings{SiteName: row.SiteName, Tagline: row.Tagline}, nil
}

func (s *Service) Update(ctx context.Context, siteName, tagline string) (Settings, error) {
	siteName = strings.TrimSpace(siteName)
	tagline = strings.TrimSpace(tagline)
	if n := utf8.RuneCountInString(siteName); n < 1 || n > 80 {
		return Settings{}, app.Invalid("站名需要 1 到 80 个字")
	}
	if utf8.RuneCountInString(tagline) > 200 {
		return Settings{}, app.Invalid("一句话介绍不能超过 200 个字")
	}
	row, err := s.q.UpdateSiteSettings(ctx, sqlc.UpdateSiteSettingsParams{SiteName: siteName, Tagline: tagline})
	if err != nil {
		return Settings{}, err
	}
	return Settings{SiteName: row.SiteName, Tagline: row.Tagline}, nil
}
