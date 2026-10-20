-- name: GetSiteSettings :one
select site_name, tagline from site_settings order by id asc limit 1;

-- name: UpdateSiteSettings :one
update site_settings
set site_name = $1, tagline = $2, updated_at = now()
where id = (select id from site_settings order by id asc limit 1)
returning site_name, tagline;
