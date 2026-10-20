package content

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/z123-piad/houttuynia-blog/internal/app"
	"github.com/z123-piad/houttuynia-blog/internal/identity"
	"github.com/z123-piad/houttuynia-blog/internal/sqlc"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type Ref struct {
	ID   int64  `json:"id,omitempty"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type PublicPost struct {
	Slug        string     `json:"slug"`
	Title       string     `json:"title"`
	Excerpt     string     `json:"excerpt"`
	Body        string     `json:"body,omitempty"`
	PublishedAt *time.Time `json:"published_at"`
	AuthorName  string     `json:"author_name"`
	Category    *Ref       `json:"category,omitempty"`
	Tags        []Ref      `json:"tags"`
}

type AdminPost struct {
	ID          int64      `json:"id"`
	AuthorID    int64      `json:"author_id"`
	AuthorName  string     `json:"author_name,omitempty"`
	CategoryID  *int64     `json:"category_id"`
	Slug        string     `json:"slug"`
	Title       string     `json:"title"`
	Excerpt     string     `json:"excerpt,omitempty"`
	Body        string     `json:"body,omitempty"`
	Status      string     `json:"status"`
	TagIDs      []int64    `json:"tag_ids,omitempty"`
	PublishedAt *time.Time `json:"published_at"`
	ViewCount   int64      `json:"view_count,omitempty"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type Category struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	SortOrder   int32  `json:"sort_order"`
}

type Tag struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type ArchiveMonth struct {
	Month string       `json:"month"`
	Posts []PublicPost `json:"posts"`
}

type PostInput struct {
	Title      string
	Slug       string
	Excerpt    string
	Body       string
	CategoryID *int64
	TagIDs     []int64
}

type Service struct {
	q    *sqlc.Queries
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{q: sqlc.New(pool), pool: pool}
}

func (s *Service) ListPublished(ctx context.Context, limit, offset int32) ([]PublicPost, error) {
	rows, err := s.q.ListPublishedPosts(ctx, sqlc.ListPublishedPostsParams{Lim: limit, Off: offset})
	if err != nil {
		return nil, err
	}
	posts := make([]PublicPost, 0, len(rows))
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		posts = append(posts, PublicPost{
			Slug:        row.Slug,
			Title:       row.Title,
			Excerpt:     row.Excerpt,
			PublishedAt: row.PublishedAt,
			AuthorName:  row.AuthorName,
			Category:    refFrom(row.CategorySlug, row.CategoryName),
			Tags:        []Ref{},
		})
		ids = append(ids, row.ID)
	}
	return s.attachPublicTags(ctx, ids, posts)
}

func (s *Service) GetPublished(ctx context.Context, slug string) (PublicPost, error) {
	row, err := s.q.GetPublishedPostBySlug(ctx, slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return PublicPost{}, app.NotFound("文章不存在")
	}
	if err != nil {
		return PublicPost{}, err
	}
	post := PublicPost{
		Slug:        row.Slug,
		Title:       row.Title,
		Excerpt:     row.Excerpt,
		Body:        row.BodyMd,
		PublishedAt: row.PublishedAt,
		AuthorName:  row.AuthorName,
		Category:    refFrom(row.CategorySlug, row.CategoryName),
		Tags:        []Ref{},
	}
	posts, err := s.attachPublicTags(ctx, []int64{row.ID}, []PublicPost{post})
	if err != nil {
		return PublicPost{}, err
	}
	return posts[0], nil
}

func (s *Service) ListByCategory(ctx context.Context, slug string) (Category, []PublicPost, error) {
	cat, err := s.q.GetCategoryBySlug(ctx, slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return Category{}, nil, app.NotFound("分类不存在")
	}
	if err != nil {
		return Category{}, nil, err
	}
	rows, err := s.q.ListPublishedByCategorySlug(ctx, slug)
	if err != nil {
		return Category{}, nil, err
	}
	posts := make([]PublicPost, 0, len(rows))
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		posts = append(posts, PublicPost{
			Slug:        row.Slug,
			Title:       row.Title,
			Excerpt:     row.Excerpt,
			PublishedAt: row.PublishedAt,
			AuthorName:  row.AuthorName,
			Category:    &Ref{Name: row.CategoryName, Slug: row.CategorySlug},
			Tags:        []Ref{},
		})
		ids = append(ids, row.ID)
	}
	posts, err = s.attachPublicTags(ctx, ids, posts)
	if err != nil {
		return Category{}, nil, err
	}
	return categoryFrom(cat.ID, cat.Name, cat.Slug, cat.Description, cat.SortOrder), posts, nil
}

func (s *Service) ListByTag(ctx context.Context, slug string) (Tag, []PublicPost, error) {
	tag, err := s.q.GetTagBySlug(ctx, slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return Tag{}, nil, app.NotFound("标签不存在")
	}
	if err != nil {
		return Tag{}, nil, err
	}
	rows, err := s.q.ListPublishedByTagSlug(ctx, slug)
	if err != nil {
		return Tag{}, nil, err
	}
	posts := make([]PublicPost, 0, len(rows))
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		posts = append(posts, PublicPost{
			Slug:        row.Slug,
			Title:       row.Title,
			Excerpt:     row.Excerpt,
			PublishedAt: row.PublishedAt,
			AuthorName:  row.AuthorName,
			Category:    refFrom(row.CategorySlug, row.CategoryName),
			Tags:        []Ref{},
		})
		ids = append(ids, row.ID)
	}
	posts, err = s.attachPublicTags(ctx, ids, posts)
	if err != nil {
		return Tag{}, nil, err
	}
	return Tag{ID: tag.ID, Name: tag.Name, Slug: tag.Slug}, posts, nil
}

func (s *Service) Search(ctx context.Context, query string) ([]PublicPost, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, app.Invalid("请输入搜索词")
	}
	if utf8.RuneCountInString(query) > 100 {
		return nil, app.Invalid("搜索词不能超过 100 个字")
	}
	stripped := strings.NewReplacer(`\`, "", "%", "", "_", "").Replace(query)
	if stripped == "" {
		return []PublicPost{}, nil
	}
	rows, err := s.q.SearchPublishedPosts(ctx, sqlc.SearchPublishedPostsParams{
		Pattern: "%" + stripped + "%",
		Lim:     50,
	})
	if err != nil {
		return nil, err
	}
	posts := make([]PublicPost, 0, len(rows))
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		posts = append(posts, PublicPost{
			Slug:        row.Slug,
			Title:       row.Title,
			Excerpt:     row.Excerpt,
			PublishedAt: row.PublishedAt,
			AuthorName:  row.AuthorName,
			Category:    refFrom(row.CategorySlug, row.CategoryName),
			Tags:        []Ref{},
		})
		ids = append(ids, row.ID)
	}
	return s.attachPublicTags(ctx, ids, posts)
}

func (s *Service) Archive(ctx context.Context) ([]ArchiveMonth, error) {
	rows, err := s.q.ListArchivePosts(ctx)
	if err != nil {
		return nil, err
	}
	var months []ArchiveMonth
	index := map[string]int{}
	for _, row := range rows {
		if row.PublishedAt == nil {
			continue
		}
		key := row.PublishedAt.UTC().Format("2006-01")
		i, ok := index[key]
		if !ok {
			index[key] = len(months)
			months = append(months, ArchiveMonth{Month: key})
			i = index[key]
		}
		months[i].Posts = append(months[i].Posts, PublicPost{
			Slug:        row.Slug,
			Title:       row.Title,
			PublishedAt: row.PublishedAt,
			Tags:        []Ref{},
		})
	}
	return months, nil
}

func (s *Service) ListCategories(ctx context.Context) ([]Category, error) {
	rows, err := s.q.ListCategories(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Category, 0, len(rows))
	for _, row := range rows {
		out = append(out, categoryFrom(row.ID, row.Name, row.Slug, row.Description, row.SortOrder))
	}
	return out, nil
}

func (s *Service) ListTags(ctx context.Context) ([]Tag, error) {
	rows, err := s.q.ListTags(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Tag, 0, len(rows))
	for _, row := range rows {
		out = append(out, Tag{ID: row.ID, Name: row.Name, Slug: row.Slug})
	}
	return out, nil
}

func (s *Service) ListAdmin(ctx context.Context, actor identity.Principal, limit, offset int32) ([]AdminPost, error) {
	authorID := int64(0)
	if scope, ok := actor.Allows("post.read"); ok && scope == "own" {
		authorID = actor.UserID
	}
	rows, err := s.q.ListAdminPosts(ctx, sqlc.ListAdminPostsParams{AuthorID: authorID, Lim: limit, Off: offset})
	if err != nil {
		return nil, err
	}
	out := make([]AdminPost, 0, len(rows))
	for _, row := range rows {
		out = append(out, AdminPost{
			ID:         row.ID,
			AuthorID:   row.AuthorID,
			AuthorName: row.AuthorName,
			Slug:       row.Slug,
			Title:      row.Title,
			Status:     row.Status,
			UpdatedAt:  row.UpdatedAt,
		})
	}
	return out, nil
}

func (s *Service) GetAdmin(ctx context.Context, actor identity.Principal, id int64) (AdminPost, error) {
	row, err := s.q.GetPostByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return AdminPost{}, app.NotFound("文章不存在")
	}
	if err != nil {
		return AdminPost{}, err
	}
	if !actor.Can("post.read", row.AuthorID) {
		return AdminPost{}, app.Forbidden("无权查看这篇文章")
	}
	tags, err := s.q.ListPostTags(ctx, id)
	if err != nil {
		return AdminPost{}, err
	}
	ids := make([]int64, 0, len(tags))
	for _, tag := range tags {
		ids = append(ids, tag.ID)
	}
	created := row.CreatedAt
	return AdminPost{
		ID:          row.ID,
		AuthorID:    row.AuthorID,
		CategoryID:  int64Ptr(row.CategoryID),
		Slug:        row.Slug,
		Title:       row.Title,
		Excerpt:     row.Excerpt,
		Body:        row.BodyMd,
		Status:      row.Status,
		TagIDs:      ids,
		PublishedAt: row.PublishedAt,
		ViewCount:   row.ViewCount,
		CreatedAt:   &created,
		UpdatedAt:   row.UpdatedAt,
	}, nil
}

func (s *Service) Create(ctx context.Context, actor identity.Principal, in PostInput) (AdminPost, error) {
	if !actor.Has("post.create") {
		return AdminPost{}, app.Forbidden("无权创建文章")
	}
	normalized, err := normalizePost(in)
	if err != nil {
		return AdminPost{}, err
	}
	var created sqlc.CreatePostRow
	err = s.tx(ctx, func(q *sqlc.Queries) error {
		if err := ensureTags(ctx, q, normalized.TagIDs); err != nil {
			return err
		}
		row, err := q.CreatePost(ctx, sqlc.CreatePostParams{
			AuthorID:   actor.UserID,
			CategoryID: optID(normalized.CategoryID),
			Slug:       normalized.Slug,
			Title:      normalized.Title,
			Excerpt:    normalized.Excerpt,
			BodyMd:     normalized.Body,
		})
		if err != nil {
			return mapPostWrite(err)
		}
		if err := replaceTags(ctx, q, row.ID, normalized.TagIDs); err != nil {
			return err
		}
		created = row
		return nil
	})
	if err != nil {
		return AdminPost{}, err
	}
	return s.GetAdmin(ctx, actor, created.ID)
}

func (s *Service) Update(ctx context.Context, actor identity.Principal, id int64, in PostInput) (AdminPost, error) {
	normalized, err := normalizePost(in)
	if err != nil {
		return AdminPost{}, err
	}
	err = s.tx(ctx, func(q *sqlc.Queries) error {
		current, err := q.GetPostByID(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return app.NotFound("文章不存在")
		}
		if err != nil {
			return err
		}
		if !actor.Can("post.update", current.AuthorID) {
			return app.Forbidden("无权修改这篇文章")
		}
		if err := ensureTags(ctx, q, normalized.TagIDs); err != nil {
			return err
		}
		if _, err := q.UpdatePost(ctx, sqlc.UpdatePostParams{
			ID:         id,
			CategoryID: optID(normalized.CategoryID),
			Slug:       normalized.Slug,
			Title:      normalized.Title,
			Excerpt:    normalized.Excerpt,
			BodyMd:     normalized.Body,
		}); err != nil {
			return mapPostWrite(err)
		}
		return replaceTags(ctx, q, id, normalized.TagIDs)
	})
	if err != nil {
		return AdminPost{}, err
	}
	return s.GetAdmin(ctx, actor, id)
}

func (s *Service) Publish(ctx context.Context, actor identity.Principal, id int64) (AdminPost, error) {
	err := s.tx(ctx, func(q *sqlc.Queries) error {
		current, err := q.GetPostByID(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return app.NotFound("文章不存在")
		}
		if err != nil {
			return err
		}
		if !actor.Can("post.publish", current.AuthorID) {
			return app.Forbidden("无权发布这篇文章")
		}
		if current.Status == "published" {
			return nil
		}
		_, err = q.PublishPost(ctx, id)
		return err
	})
	if err != nil {
		return AdminPost{}, err
	}
	return s.GetAdmin(ctx, actor, id)
}

func (s *Service) SoftDelete(ctx context.Context, actor identity.Principal, id int64) error {
	current, err := s.q.GetPostByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return app.NotFound("文章不存在")
	}
	if err != nil {
		return err
	}
	if !actor.Can("post.delete", current.AuthorID) {
		return app.Forbidden("无权删除这篇文章")
	}
	n, err := s.q.SoftDeletePost(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return app.NotFound("文章不存在")
	}
	return nil
}

func (s *Service) CreateCategory(ctx context.Context, name, slug, description string, sortOrder int32) (Category, error) {
	name, slug, description, err := normalizeNameSlug(name, slug, description, 80, 500)
	if err != nil {
		return Category{}, err
	}
	if sortOrder < -1000 || sortOrder > 10000 {
		return Category{}, app.Invalid("排序需要在 -1000 到 10000 之间")
	}
	row, err := s.q.CreateCategory(ctx, sqlc.CreateCategoryParams{
		Name: name, Slug: slug, Description: description, SortOrder: sortOrder,
	})
	if app.UniqueViolation(err) {
		return Category{}, app.Conflict("标识已被使用")
	}
	if err != nil {
		return Category{}, err
	}
	return categoryFrom(row.ID, row.Name, row.Slug, row.Description, row.SortOrder), nil
}

func (s *Service) UpdateCategory(ctx context.Context, id int64, name, slug, description string, sortOrder int32) (Category, error) {
	name, slug, description, err := normalizeNameSlug(name, slug, description, 80, 500)
	if err != nil {
		return Category{}, err
	}
	if sortOrder < -1000 || sortOrder > 10000 {
		return Category{}, app.Invalid("排序需要在 -1000 到 10000 之间")
	}
	row, err := s.q.UpdateCategory(ctx, sqlc.UpdateCategoryParams{
		ID: id, Name: name, Slug: slug, Description: description, SortOrder: sortOrder,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Category{}, app.NotFound("分类不存在")
	}
	if app.UniqueViolation(err) {
		return Category{}, app.Conflict("标识已被使用")
	}
	if err != nil {
		return Category{}, err
	}
	return categoryFrom(row.ID, row.Name, row.Slug, row.Description, row.SortOrder), nil
}

func (s *Service) DeleteCategory(ctx context.Context, id int64) error {
	n, err := s.q.DeleteCategory(ctx, id)
	if app.ForeignKeyViolation(err) {
		return app.Conflict("仍有文章使用该分类")
	}
	if err != nil {
		return err
	}
	if n == 0 {
		return app.NotFound("分类不存在")
	}
	return nil
}

func (s *Service) CreateTag(ctx context.Context, name, slug string) (Tag, error) {
	name, slug, _, err := normalizeNameSlug(name, slug, "", 40, 0)
	if err != nil {
		return Tag{}, err
	}
	row, err := s.q.CreateTag(ctx, sqlc.CreateTagParams{Name: name, Slug: slug})
	if app.UniqueViolation(err) {
		return Tag{}, app.Conflict("标识已被使用")
	}
	if err != nil {
		return Tag{}, err
	}
	return Tag{ID: row.ID, Name: row.Name, Slug: row.Slug}, nil
}

func (s *Service) UpdateTag(ctx context.Context, id int64, name, slug string) (Tag, error) {
	name, slug, _, err := normalizeNameSlug(name, slug, "", 40, 0)
	if err != nil {
		return Tag{}, err
	}
	row, err := s.q.UpdateTag(ctx, sqlc.UpdateTagParams{ID: id, Name: name, Slug: slug})
	if errors.Is(err, pgx.ErrNoRows) {
		return Tag{}, app.NotFound("标签不存在")
	}
	if app.UniqueViolation(err) {
		return Tag{}, app.Conflict("标识已被使用")
	}
	if err != nil {
		return Tag{}, err
	}
	return Tag{ID: row.ID, Name: row.Name, Slug: row.Slug}, nil
}

func (s *Service) DeleteTag(ctx context.Context, id int64) error {
	if err := s.q.DeleteTagLinks(ctx, id); err != nil {
		return err
	}
	n, err := s.q.DeleteTag(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return app.NotFound("标签不存在")
	}
	return nil
}

func (s *Service) tx(ctx context.Context, fn func(q *sqlc.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(s.q.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) attachPublicTags(ctx context.Context, ids []int64, posts []PublicPost) ([]PublicPost, error) {
	if len(ids) == 0 {
		return posts, nil
	}
	rows, err := s.q.ListTagsForPosts(ctx, ids)
	if err != nil {
		return nil, err
	}
	byPost := map[int64][]Ref{}
	for _, row := range rows {
		byPost[row.PostID] = append(byPost[row.PostID], Ref{ID: row.ID, Name: row.Name, Slug: row.Slug})
	}
	for i, id := range ids {
		if tags := byPost[id]; tags != nil {
			posts[i].Tags = tags
		}
	}
	return posts, nil
}

func ensureTags(ctx context.Context, q *sqlc.Queries, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	n, err := q.CountTagsByIDs(ctx, ids)
	if err != nil {
		return err
	}
	if n != int64(len(uniqueIDs(ids))) {
		return app.Invalid("有标签不存在")
	}
	return nil
}

func replaceTags(ctx context.Context, q *sqlc.Queries, postID int64, ids []int64) error {
	if err := q.DeletePostTags(ctx, postID); err != nil {
		return err
	}
	seen := map[int64]struct{}{}
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		if err := q.InsertPostTag(ctx, sqlc.InsertPostTagParams{PostID: postID, TagID: id}); err != nil {
			return err
		}
	}
	return nil
}

func uniqueIDs(ids []int64) []int64 {
	seen := map[int64]struct{}{}
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func normalizePost(in PostInput) (PostInput, error) {
	in.Title = strings.TrimSpace(in.Title)
	in.Slug = strings.TrimSpace(in.Slug)
	in.Excerpt = strings.TrimSpace(in.Excerpt)
	in.Body = strings.TrimSpace(in.Body)
	if n := utf8.RuneCountInString(in.Title); n < 1 || n > 200 {
		return PostInput{}, app.Invalid("标题需要 1 到 200 个字")
	}
	if n := utf8.RuneCountInString(in.Slug); n < 1 || n > 80 || !slugPattern.MatchString(in.Slug) {
		return PostInput{}, app.Invalid("标识只能使用小写字母、数字和连字符")
	}
	if utf8.RuneCountInString(in.Excerpt) > 500 {
		return PostInput{}, app.Invalid("摘要不能超过 500 个字")
	}
	if utf8.RuneCountInString(in.Body) > 200000 {
		return PostInput{}, app.Invalid("正文过长")
	}
	return in, nil
}

func normalizeNameSlug(name, slug, description string, nameMax, descMax int) (string, string, string, error) {
	name = strings.TrimSpace(name)
	slug = strings.TrimSpace(slug)
	description = strings.TrimSpace(description)
	if n := utf8.RuneCountInString(name); n < 1 || n > nameMax {
		return "", "", "", app.Invalid("名称长度不正确")
	}
	if n := utf8.RuneCountInString(slug); n < 1 || n > 80 || !slugPattern.MatchString(slug) {
		return "", "", "", app.Invalid("标识只能使用小写字母、数字和连字符")
	}
	if descMax > 0 && utf8.RuneCountInString(description) > descMax {
		return "", "", "", app.Invalid("说明过长")
	}
	return name, slug, description, nil
}

func mapPostWrite(err error) error {
	if err == nil {
		return nil
	}
	if app.ForeignKeyViolation(err) {
		return app.Invalid("分类不存在")
	}
	if app.UniqueViolation(err) {
		return app.Conflict("标识已被使用")
	}
	return err
}

func refFrom(slug, name *string) *Ref {
	if slug == nil || name == nil || *slug == "" {
		return nil
	}
	return &Ref{Name: *name, Slug: *slug}
}

func categoryFrom(id int64, name, slug, description string, sortOrder int32) Category {
	return Category{ID: id, Name: name, Slug: slug, Description: description, SortOrder: sortOrder}
}

func optID(id *int64) pgtype.Int8 {
	if id == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *id, Valid: true}
}

func int64Ptr(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	n := v.Int64
	return &n
}
