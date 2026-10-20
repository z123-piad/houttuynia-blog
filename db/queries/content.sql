-- name: ListPublishedPosts :many
select
    p.id,
    p.slug,
    p.title,
    p.excerpt,
    p.published_at,
    u.display_name as author_name,
    c.slug as category_slug,
    c.name as category_name
from posts p
join users u on u.id = p.author_id
left join categories c on c.id = p.category_id
where p.deleted_at is null and p.status = 'published'
order by p.published_at desc nulls last, p.id desc
limit sqlc.arg(lim) offset sqlc.arg(off);

-- name: GetPublishedPostBySlug :one
select
    p.id,
    p.slug,
    p.title,
    p.excerpt,
    p.body_md,
    p.published_at,
    u.display_name as author_name,
    c.slug as category_slug,
    c.name as category_name
from posts p
join users u on u.id = p.author_id
left join categories c on c.id = p.category_id
where p.deleted_at is null and p.status = 'published' and p.slug = $1;

-- name: ListPublishedByCategorySlug :many
select
    p.id,
    p.slug,
    p.title,
    p.excerpt,
    p.published_at,
    u.display_name as author_name,
    c.slug as category_slug,
    c.name as category_name
from posts p
join users u on u.id = p.author_id
join categories c on c.id = p.category_id
where p.deleted_at is null and p.status = 'published' and c.slug = $1
order by p.published_at desc nulls last, p.id desc;

-- name: ListPublishedByTagSlug :many
select
    p.id,
    p.slug,
    p.title,
    p.excerpt,
    p.published_at,
    u.display_name as author_name,
    c.slug as category_slug,
    c.name as category_name
from posts p
join users u on u.id = p.author_id
join post_tags pt on pt.post_id = p.id
join tags t on t.id = pt.tag_id
left join categories c on c.id = p.category_id
where p.deleted_at is null and p.status = 'published' and t.slug = $1
order by p.published_at desc nulls last, p.id desc;

-- name: SearchPublishedPosts :many
select
    p.id,
    p.slug,
    p.title,
    p.excerpt,
    p.published_at,
    u.display_name as author_name,
    c.slug as category_slug,
    c.name as category_name
from posts p
join users u on u.id = p.author_id
left join categories c on c.id = p.category_id
where p.deleted_at is null
  and p.status = 'published'
  and (p.title ilike sqlc.arg(pattern) or p.excerpt ilike sqlc.arg(pattern))
order by p.published_at desc nulls last, p.id desc
limit sqlc.arg(lim);

-- name: ListArchivePosts :many
select p.slug, p.title, p.published_at
from posts p
where p.deleted_at is null
  and p.status = 'published'
  and p.published_at is not null
order by p.published_at desc, p.id desc;

-- name: ListAdminPosts :many
select
    p.id,
    p.slug,
    p.title,
    p.status,
    p.author_id,
    p.updated_at,
    u.display_name as author_name
from posts p
join users u on u.id = p.author_id
where p.deleted_at is null
  and (sqlc.arg(author_id)::bigint = 0 or p.author_id = sqlc.arg(author_id))
order by p.updated_at desc, p.id desc
limit sqlc.arg(lim) offset sqlc.arg(off);

-- name: GetPostByID :one
select
    p.id,
    p.author_id,
    p.category_id,
    p.slug,
    p.title,
    p.excerpt,
    p.body_md,
    p.status,
    p.published_at,
    p.view_count,
    p.created_at,
    p.updated_at
from posts p
where p.id = $1 and p.deleted_at is null;

-- name: CreatePost :one
insert into posts (author_id, category_id, slug, title, excerpt, body_md, status)
values ($1, $2, $3, $4, $5, $6, 'draft')
returning id, author_id, category_id, slug, title, excerpt, body_md, status, published_at, view_count, created_at, updated_at;

-- name: UpdatePost :one
update posts
set category_id = $2,
    slug = $3,
    title = $4,
    excerpt = $5,
    body_md = $6,
    updated_at = now()
where id = $1 and deleted_at is null
returning id, author_id, category_id, slug, title, excerpt, body_md, status, published_at, view_count, created_at, updated_at;

-- name: PublishPost :one
update posts
set status = 'published',
    published_at = coalesce(published_at, now()),
    updated_at = now()
where id = $1 and deleted_at is null
returning id, author_id, category_id, slug, title, excerpt, body_md, status, published_at, view_count, created_at, updated_at;

-- name: SoftDeletePost :execrows
update posts
set deleted_at = now(), updated_at = now()
where id = $1 and deleted_at is null;

-- name: DeletePostTags :exec
delete from post_tags where post_id = $1;

-- name: InsertPostTag :exec
insert into post_tags (post_id, tag_id) values ($1, $2);

-- name: ListPostTags :many
select t.id, t.name, t.slug
from post_tags pt
join tags t on t.id = pt.tag_id
where pt.post_id = $1
order by t.name asc, t.id asc;

-- name: ListTagsForPosts :many
select pt.post_id, t.id, t.name, t.slug
from post_tags pt
join tags t on t.id = pt.tag_id
where pt.post_id = any(sqlc.arg(post_ids)::bigint[])
order by t.name asc, t.id asc;

-- name: CountTagsByIDs :one
select count(*)::bigint
from tags
where id = any(sqlc.arg(tag_ids)::bigint[]);

-- name: ListCategories :many
select id, name, slug, description, sort_order
from categories
order by sort_order asc, id asc;

-- name: GetCategoryByID :one
select id, name, slug, description, sort_order
from categories
where id = $1;

-- name: GetCategoryBySlug :one
select id, name, slug, description, sort_order
from categories
where slug = $1;

-- name: CreateCategory :one
insert into categories (name, slug, description, sort_order)
values ($1, $2, $3, $4)
returning id, name, slug, description, sort_order;

-- name: UpdateCategory :one
update categories
set name = $2, slug = $3, description = $4, sort_order = $5, updated_at = now()
where id = $1
returning id, name, slug, description, sort_order;

-- name: DeleteCategory :execrows
delete from categories where id = $1;

-- name: ListTags :many
select id, name, slug from tags order by name asc, id asc;

-- name: GetTagByID :one
select id, name, slug from tags where id = $1;

-- name: GetTagBySlug :one
select id, name, slug from tags where slug = $1;

-- name: CreateTag :one
insert into tags (name, slug) values ($1, $2)
returning id, name, slug;

-- name: UpdateTag :one
update tags set name = $2, slug = $3, updated_at = now()
where id = $1
returning id, name, slug;

-- name: DeleteTag :execrows
delete from tags where id = $1;

-- name: DeleteTagLinks :exec
delete from post_tags where tag_id = $1;
