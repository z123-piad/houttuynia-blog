-- name: GetUserByEmail :one
select id, email, password_hash, display_name, status, authz_version, deleted_at
from users
where email = $1 and deleted_at is null;

-- name: GetUserByID :one
select id, email, password_hash, display_name, status, authz_version, deleted_at
from users
where id = $1 and deleted_at is null;

-- name: CreateUser :one
insert into users (email, password_hash, display_name, status)
values ($1, $2, $3, 'active')
returning id, email, password_hash, display_name, status, authz_version, deleted_at;

-- name: GetRoleByName :one
select id, name, builtin from roles where name = $1;

-- name: AssignUserRole :exec
insert into user_roles (user_id, role_id) values ($1, $2)
on conflict (user_id, role_id) do nothing;

-- name: CreateUserIdentity :exec
insert into user_identities (
    user_id, provider, app_id, provider_user_id, union_id, raw
) values ($1, $2, $3, $4, $5, $6);

-- name: HasPasswordIdentity :one
select exists(
    select 1 from user_identities
    where user_id = $1 and provider = 'password' and app_id = 'local'
) as ok;

-- name: CreateSession :one
insert into sessions (user_id, token_hash, expires_at, authz_version)
values ($1, $2, $3, $4)
returning id, user_id, token_hash, expires_at, authz_version;

-- name: GetSessionByTokenHash :one
select
    s.id,
    s.user_id,
    s.token_hash,
    s.expires_at,
    s.authz_version,
    u.email,
    u.display_name,
    u.status,
    u.authz_version as user_authz_version,
    u.deleted_at
from sessions s
join users u on u.id = s.user_id
where s.token_hash = $1;

-- name: UpdateSessionAuthzVersion :exec
update sessions
set authz_version = $2, updated_at = now()
where id = $1;

-- name: DeleteSessionByTokenHash :exec
delete from sessions where token_hash = $1;

-- name: ListUserPermissionScopes :many
select p.code, rp.scope
from user_roles ur
join role_permissions rp on rp.role_id = ur.role_id
join permissions p on p.id = rp.permission_id
where ur.user_id = $1;

-- name: ListUserRoleNames :many
select r.name
from user_roles ur
join roles r on r.id = ur.role_id
where ur.user_id = $1
order by r.name;

-- name: ListMenus :many
select id, name, route, parent_id, permission_code, sort_order
from menus
order by sort_order asc, id asc;

-- name: CountActiveSuperAdmins :one
select count(*)::bigint
from users u
join user_roles ur on ur.user_id = u.id
join roles r on r.id = ur.role_id
where r.name = 'super_admin' and u.deleted_at is null;

-- name: BumpAuthzVersionForRole :exec
update users u
set authz_version = u.authz_version + 1, updated_at = now()
where u.deleted_at is null
  and exists (
      select 1 from user_roles ur
      where ur.user_id = u.id and ur.role_id = $1
  );
