-- houttuynia-blog schema. Primary keys are bigint identity except view_daily,
-- whose key is (post_id, view_date) as specified.

create table users (
    id bigint generated always as identity primary key,
    email text not null,
    password_hash text,
    display_name text not null,
    status text not null default 'active',
    authz_version integer not null default 1,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    deleted_at timestamptz,
    constraint users_status_chk check (status in ('active', 'disabled')),
    constraint users_authz_version_chk check (authz_version >= 1)
);

create unique index users_email_active_uidx on users (email) where deleted_at is null;

create table roles (
    id bigint generated always as identity primary key,
    name text not null unique,
    builtin boolean not null default false,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table permissions (
    id bigint generated always as identity primary key,
    code text not null unique,
    description text not null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table role_permissions (
    role_id bigint not null references roles (id),
    permission_id bigint not null references permissions (id),
    scope text not null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    primary key (role_id, permission_id),
    constraint role_permissions_scope_chk check (scope in ('any', 'own'))
);

create table user_roles (
    user_id bigint not null references users (id),
    role_id bigint not null references roles (id),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    primary key (user_id, role_id)
);

create table menus (
    id bigint generated always as identity primary key,
    name text not null,
    route text not null,
    parent_id bigint references menus (id),
    permission_code text not null,
    sort_order integer not null default 0,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table sessions (
    id bigint generated always as identity primary key,
    user_id bigint not null references users (id),
    token_hash text not null unique,
    expires_at timestamptz not null,
    authz_version integer not null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create index sessions_user_id_idx on sessions (user_id);

create table categories (
    id bigint generated always as identity primary key,
    name text not null,
    slug text not null unique,
    description text not null default '',
    sort_order integer not null default 0,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table tags (
    id bigint generated always as identity primary key,
    name text not null,
    slug text not null unique,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table posts (
    id bigint generated always as identity primary key,
    author_id bigint not null references users (id),
    category_id bigint references categories (id),
    slug text not null,
    title text not null,
    excerpt text not null default '',
    body_md text not null default '',
    status text not null default 'draft',
    published_at timestamptz,
    view_count bigint not null default 0,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    deleted_at timestamptz,
    constraint posts_status_chk check (status in ('draft', 'published', 'archived')),
    constraint posts_view_count_chk check (view_count >= 0)
);

create unique index posts_slug_active_uidx on posts (slug) where deleted_at is null;
create index posts_published_idx on posts (published_at desc) where deleted_at is null and status = 'published';
create index posts_author_idx on posts (author_id) where deleted_at is null;

create table post_tags (
    post_id bigint not null references posts (id),
    tag_id bigint not null references tags (id),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    primary key (post_id, tag_id)
);

create table assets (
    id bigint generated always as identity primary key,
    storage text not null,
    bucket text,
    object_key text not null,
    mime text not null,
    size bigint not null,
    owner_id bigint not null references users (id),
    visibility text not null,
    checksum text not null default '',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    deleted_at timestamptz,
    constraint assets_storage_chk check (storage in ('qiniu', 's3', 'local')),
    constraint assets_visibility_chk check (visibility in ('public', 'private')),
    constraint assets_size_chk check (size >= 0)
);

create index assets_owner_idx on assets (owner_id) where deleted_at is null;
create index assets_object_idx on assets (storage, object_key) where deleted_at is null;

create table site_settings (
    id bigint generated always as identity primary key,
    site_name text not null,
    tagline text not null default '',
    only_one boolean not null default true,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    constraint site_settings_singleton_chk check (only_one),
    constraint site_settings_singleton_uidx unique (only_one)
);

create table comments (
    id bigint generated always as identity primary key,
    post_id bigint not null references posts (id),
    user_id bigint references users (id),
    parent_id bigint references comments (id),
    body text not null,
    status text not null default 'pending',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    deleted_at timestamptz,
    constraint comments_status_chk check (status in ('pending', 'approved', 'spam'))
);

create index comments_post_idx on comments (post_id);

create table friend_links (
    id bigint generated always as identity primary key,
    name text not null,
    url text not null,
    description text not null default '',
    status text not null default 'pending',
    sort_order integer not null default 0,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    constraint friend_links_status_chk check (status in ('pending', 'approved', 'hidden'))
);

create table view_daily (
    post_id bigint not null references posts (id),
    view_date date not null,
    view_count bigint not null default 0,
    visitor_hashes jsonb not null default '[]'::jsonb,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    primary key (post_id, view_date),
    constraint view_daily_count_chk check (view_count >= 0)
);

create table profiles (
    id bigint generated always as identity primary key,
    user_id bigint not null unique references users (id),
    bio text not null default '',
    avatar_asset_id bigint references assets (id),
    social_links jsonb not null default '{}'::jsonb,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table user_identities (
    id bigint generated always as identity primary key,
    user_id bigint not null references users (id),
    provider text not null,
    app_id text not null,
    provider_user_id text not null,
    union_id text,
    access_token text,
    refresh_token text,
    token_expires_at timestamptz,
    raw jsonb not null default '{}'::jsonb,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    constraint user_identities_provider_chk check (provider in ('password', 'wechat', 'qq', 'dingtalk', 'feishu')),
    constraint user_identities_provider_uidx unique (provider, app_id, provider_user_id),
    constraint user_identities_wechat_union_chk check (
        provider <> 'wechat' or (union_id is not null and length(btrim(union_id)) > 0)
    )
);

create index user_identities_wechat_union_idx on user_identities (union_id) where provider = 'wechat';

create or replace function ensure_active_super_admin()
returns trigger
language plpgsql
as $$
declare
    remaining integer;
begin
    select count(*) into remaining
    from users u
    join user_roles ur on ur.user_id = u.id
    join roles r on r.id = ur.role_id
    where r.name = 'super_admin'
      and u.deleted_at is null;
    if remaining < 1 then
        raise exception 'at least one super_admin must remain';
    end if;
    return null;
end;
$$;

create constraint trigger trg_users_keep_super_admin
after update of deleted_at on users
deferrable initially deferred
for each row
when (old.deleted_at is null and new.deleted_at is not null)
execute function ensure_active_super_admin();

create constraint trigger trg_users_delete_keep_super_admin
after delete on users
deferrable initially deferred
for each row
execute function ensure_active_super_admin();

create constraint trigger trg_user_roles_keep_super_admin
after delete or update on user_roles
deferrable initially deferred
for each row
execute function ensure_active_super_admin();

create or replace function prevent_builtin_role_delete()
returns trigger
language plpgsql
as $$
begin
    if old.builtin then
        raise exception 'builtin roles cannot be deleted';
    end if;
    return old;
end;
$$;

create trigger trg_roles_keep_builtin
before delete on roles
for each row
execute function prevent_builtin_role_delete();

insert into roles (name, builtin) values
    ('super_admin', true),
    ('admin', true),
    ('author', true),
    ('reader', true);

insert into permissions (code, description) values
    ('post.create', '创建文章'),
    ('post.read', '查看文章（含草稿）'),
    ('post.update', '修改文章'),
    ('post.delete', '删除文章'),
    ('post.publish', '发布文章'),
    ('category.manage', '管理分类'),
    ('tag.manage', '管理标签'),
    ('comment.moderate', '审核评论'),
    ('link.manage', '管理友链'),
    ('asset.upload', '上传文件'),
    ('asset.read', '读取文件'),
    ('asset.delete', '删除文件'),
    ('user.manage', '管理用户'),
    ('role.manage', '管理角色'),
    ('setting.manage', '管理站点设置'),
    ('stats.read', '查看统计'),
    ('tool.use', '使用站内工具'),
    ('profile.update', '修改个人简介'),
    ('comment.create', '发表评论');

insert into role_permissions (role_id, permission_id, scope)
select r.id, p.id, 'any'
from roles r
cross join permissions p
where r.name = 'super_admin';

insert into role_permissions (role_id, permission_id, scope)
select r.id, p.id, 'any'
from roles r
join permissions p on p.code in (
    'post.create', 'post.read', 'post.update', 'post.delete', 'post.publish',
    'category.manage', 'tag.manage', 'comment.moderate', 'link.manage',
    'asset.upload', 'asset.read', 'asset.delete', 'setting.manage'
)
where r.name = 'admin';

insert into role_permissions (role_id, permission_id, scope)
select r.id, p.id, 'own'
from roles r
join permissions p on p.code in (
    'post.create', 'post.read', 'post.update', 'post.delete',
    'asset.upload', 'asset.read', 'asset.delete'
)
where r.name = 'author';

insert into role_permissions (role_id, permission_id, scope)
select r.id, p.id, 'own'
from roles r
join permissions p on p.code in ('profile.update', 'comment.create')
where r.name = 'reader';

insert into menus (name, route, parent_id, permission_code, sort_order) values
    ('文章', '/admin/posts', null, 'post.read', 10),
    ('分类', '/admin/categories', null, 'category.manage', 20),
    ('标签', '/admin/tags', null, 'tag.manage', 30),
    ('站点设置', '/admin/settings', null, 'setting.manage', 40);

insert into site_settings (site_name, tagline) values ('houttuynia', '');
