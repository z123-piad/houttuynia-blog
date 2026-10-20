# 项目状态

houttuynia-blog（个人博客系统）。本文是架构的唯一口径。教程只解释本文，不另起技术选型。

## 版本

- 版本号：`0.1.0`
- 状态：第一版已实现。读者页、后台、Go API、RBAC、本地存储和预留表都在当前仓库里。

## 现在做到哪

- 读者：首页、文章、分类、标签、归档、标题和摘要搜索、空的关于页。关于页不读 `profiles`。
- 后台：登录和退出、文章草稿 / 编辑 / 发布 / 软删除、分类、标签、站名和一句话介绍、公开图片（先申请凭证，再确认）。
- RBAC：迁移写入四个内置角色和全部权限码。可用的超级管理员在 API 启动时按 `SEED_ADMIN_EMAIL` / `SEED_ADMIN_PASSWORD` 种子，因为 SQL 迁移不能对密码做 argon2id。该邮箱已存在时不重置密码。
- `local` 可以上传、公开访问，并在 `storage.Service.SignedURL` 签发后从 `/media-signed` 下载。`qiniu` 和 `s3` 实现同一接口；密钥不全时返回错误，不假装成功。
- 私有文件的短时链接方法已实现。签发这条链接的 HTTP 路由不注册，浏览器预览库也不做。
- `/tools` 是懒加载空位，不调用 API。

评论、友链、`profiles`、`view_daily`、`posts.view_count` 仍只有表或包，没有页面和业务路由。三方登录只有 `user_identities` 和 Provider 接口。

## 运行形态

小服务器上只有三块：

1. Nginx，或由 Go 进程托管 Vite 打出的静态文件。
2. 一个 Go API 进程。
3. PostgreSQL。

读者页和后台是同一个 React 应用，后台路由前缀 `/admin`。静态资源和 API 放在同一站点源，会话用 Cookie。不跑 Node SSR，也不为渲染页面常驻 Node 进程。

## 技术选型

### 前台

- React，用 Vite 构建成静态资源。
- 读者页与后台同一应用。后台路由 `/admin`。
- 以后的 TypeScript 单页小工具放在 `/tools`，路由懒加载。当前只在 [Tutorials/reserved.md](Tutorials/reserved.md) 写明占位方式。

正文以 Markdown 原文存放。API 返回 Markdown，由 React 渲染。

### 后端

- 语言：Go。HTTP API 前缀 `/api/v1`。
- 路由库：`github.com/go-chi/chi/v5`。
  chi 包在标准库 `net/http` 上，补充分组和中间件，依赖少，适合单个 API 进程。`/api/v1` 做成路由组，会话和权限中间件挂在组上即可。标准库 `ServeMux` 能匹配路径，分组和中间件仍要自己拼；chi 只补这一层。不换成 Gin 这类更重的框架。
- 查询：sqlc。不引入 ORM。
- 迁移：golang-migrate。
- 模块路径：`github.com/z123-piad/houttuynia-blog`。
- 领域分包（目录已经存在）：

| 包 | 职责 |
| --- | --- |
| `identity` | 用户、会话、角色、权限、菜单、身份、个人简介 |
| `content` | 文章、分类、标签 |
| `comment` | 评论 |
| `link` | 友链 |
| `stats` | 阅读统计 |
| `site` | 站点设置（站名、一句话介绍） |
| `storage` | 存储接口与 qiniu、s3、local 驱动 |

目录：`cmd/api`、`internal/<领域>`、`db/migrations`、`db/queries`、`web/`。

### 数据库

只有 PostgreSQL。

- 第三方原始资料、社交链接、当日访客哈希列表用 `jsonb`。
- 搜索用 PostgreSQL，查已发布文章的标题和摘要。
- 会话存在 PostgreSQL。

主键全库使用 `bigint generated always as identity`。文章对外地址用 `slug`，不把内部主键暴露成读者 URL。

### 文件

存储接口四个方法：上传凭证、公开 URL、私有签名 URL、删除。

驱动三个，运行时用环境变量选择一个：

- `qiniu`
- `s3`：同一套接口兼容阿里云、腾讯云、MinIO
- `local`

密钥只放环境变量，不进仓库，不打进前端包。

七牛和 S3：浏览器直传。API 签发凭证并登记元数据，不转发文件流。公开图片使用 CDN。私有文件先过权限，再发短时签名链接；预览在浏览器里做（pdf.js、SheetJS、mammoth）。

`local` 仅用于本机或单机盘。上传端点属于 storage 驱动，业务接口仍然只发凭证和登记元数据。

### 环境变量

值不入库，名称约定如下：

- `DATABASE_URL`
- `SESSION_SECRET`
- `STORAGE_DRIVER`：`qiniu`、`s3` 或 `local`
- 七牛：`QINIU_ACCESS_KEY`、`QINIU_SECRET_KEY`、`QINIU_BUCKET`、`QINIU_CDN_DOMAIN`
- S3 兼容：`S3_ENDPOINT`、`S3_REGION`、`S3_BUCKET`、`S3_ACCESS_KEY`、`S3_SECRET_KEY`
- 本地盘：`LOCAL_STORAGE_PATH`
- 种子超管：`SEED_ADMIN_EMAIL`、`SEED_ADMIN_PASSWORD`
- 统计实现时才读取：`VISITOR_HASH_KEY`（第一版不计数，可以先不配置）

## 第一版要做的功能

### 读者（匿名）

公开接口只返回 `status=published` 且 `deleted_at` 为空的文章。不检查后台权限码 `post.read`。

- 首页列表
- 文章页（按 slug）
- 分类
- 标签
- 归档（按 `published_at` 的月份）
- 搜索：标题和摘要（PostgreSQL，第一版可用 `ILIKE`；以后若要分词，仍留在 PostgreSQL）
- 关于页：路由在，内容可先空，不读 `profiles`

路由：

- `/`
- `/posts/:slug`
- `/categories/:slug`
- `/tags/:slug`
- `/archive`
- `/search`
- `/about`

### 后台 `/admin`

- 登录、退出
- 文章草稿、发布、编辑、软删除
- 分类
- 标签
- 站点设置：站名、一句话介绍
- 公开图片上传（浏览器直传）

路由：

- `/admin/login`
- `/admin/posts`
- `/admin/posts/new`
- `/admin/posts/:id/edit`
- `/admin/categories`
- `/admin/tags`
- `/admin/settings`

### 鉴权

RBAC 链路按完整模型接通：密码登录、HttpOnly 会话、权限中间件、`own` 在服务层核对 `author_id` / `owner_id`、菜单按权限下发。

种子只创建一名 `super_admin` 用户，以及全部内置角色和权限码。不另建 admin、author、reader 账号。

## 第一版不做页面和路由，但要留下的部分

表、权限码、包或接口已经留下。不写页面，不注册对应业务路由。

- 评论 `comments`
- 友链 `friend_links`
- 统计：`posts.view_count` 与 `view_daily`。不存原始访问日志，第一版不计数
- 个人简介 `profiles`（与 `users` 一对一）。关于页以后读它
- 三方登录 `user_identities` 与 Provider 接口。路由不注册。微信必须能存 `union_id`。新的三方用户默认角色 `reader`
- 站内小工具：前端 `/tools` 懒加载占位，无独立后端。要落库时用 PostgreSQL
- `site_settings` 与个人简介分开。站点设置属于第一版页面；简介只留表

密码登录会在 `user_identities` 写 `provider=password` 的一行，使本地身份和三方身份同一张表。这不等于开放三方登录。

## 数据

时间用 `timestamptz`。下面未写的审计字段，每张业务表保留 `created_at`、`updated_at`。

### users

| 字段 | 说明 |
| --- | --- |
| email | 登录邮箱。`deleted_at is null` 的行上唯一 |
| password_hash | argon2id 哈希。不存明文 |
| display_name | 显示名 |
| status | `active` 或 `disabled`。`disabled` 不能登录 |
| authz_version | 整数，从 1 起。角色或该角色的权限变化时加 1 |
| deleted_at | 软删除 |

### roles、permissions、role_permissions、user_roles

`roles`：`name`、`builtin`。内置四行不可删除：`super_admin`、`admin`、`author`、`reader`。

`permissions`：`code` 唯一，另有说明文字。

`post.*` 落实为五个码，发布不并进修改：

- `post.create`
- `post.read`
- `post.update`
- `post.delete`
- `post.publish`

其余至少包括：

- `category.manage`
- `tag.manage`
- `comment.moderate`
- `link.manage`
- `asset.upload`
- `asset.read`
- `asset.delete`
- `user.manage`
- `role.manage`
- `setting.manage`
- `stats.read`
- `tool.use`

读者角色需要、第一版没有页面的补充码：`profile.update`、`comment.create`。

`role_permissions`：`role_id`、`permission_id`、`scope`。`scope` 只有 `any` 或 `own`。

`user_roles`：用户与角色多对多。同一用户的权限取并集。同一权限码既有 `any` 又有 `own` 时，按 `any`。

内置授予：

| 权限码 | super_admin | admin | author | reader |
| --- | --- | --- | --- | --- |
| `post.create` / `post.read` / `post.update` / `post.delete` | any | any | own | — |
| `post.publish` | any | any | 默认不授予；若另授则为 own | — |
| `category.manage`、`tag.manage` | any | any | — | — |
| `comment.moderate` | any | any | — | — |
| `link.manage` | any | any | — | — |
| `asset.upload` / `asset.read` / `asset.delete` | any | any | own | — |
| `setting.manage` | any | any | — | — |
| `user.manage`、`role.manage` | any | — | — | — |
| `stats.read`、`tool.use` | any | — | — | — |
| `profile.update`、`comment.create` | any | — | — | own |

admin 管内容、评论、友链、文件和站点设置，不管用户与角色。author 的文章和文件只限本人。`post.publish` 单独存在，不包含在 author 的默认文章权限里。reader 只对应以后的本人简介和评论。

至少保留一名未删除的 `super_admin` 用户。没有部门，没有组织架构表。

公开阅读不使用 `post.read`。`post.read` 表示后台可以查看文章（含草稿）；范围是 `own` 时只能看自己的。

### menus

字段：名称、路由、父级（可空）、权限码、排序。

后台导航由 `GET /api/v1/admin/menus` 按当前用户持有的权限码下发。菜单只看有没有该码，不看 `scope`。点进具体文章或文件时，服务层再核对 `author_id` / `owner_id`。

第一版只插入已有页面：文章（`post.read`）、分类（`category.manage`）、标签（`tag.manage`）、站点设置（`setting.manage`）。预留模块的权限码入库，菜单行等页面出现再插入。

前端用这份菜单隐藏入口。服务端仍然逐个接口检查权限。

### sessions

会话行存在 PostgreSQL：`user_id`、令牌哈希、过期时间、装载权限时的 `authz_version`。

Cookie 名为会话标识，HttpOnly；生产环境加 Secure；SameSite=Lax。Cookie 里不放权限列表。

请求进入时，若 `users.authz_version` 大于会话里记下的版本，重新查询角色权限，写回会话，不强制登出。

两种情况要给受影响用户的 `authz_version` 加 1：更换该用户的角色；修改其拥有角色的 `role_permissions`。

### posts、categories、tags、post_tags

`posts`：

| 字段 | 说明 |
| --- | --- |
| author_id | 作者 |
| category_id | 一个分类，可空。一篇文章只有一个分类，不用关联表 |
| slug | `deleted_at is null` 时唯一 |
| title | 标题。搜索范围 |
| excerpt | 摘要。搜索范围 |
| body_md | Markdown 原文 |
| status | `draft`、`published`、`archived` |
| published_at | 变为 `published` 时写入。归档用它 |
| view_count | 累计阅读。列留下，第一版不增加 |
| deleted_at | 软删除 |

`categories`：名称、slug、说明、排序。`tags`：名称、slug。`post_tags`：`post_id`、`tag_id`，联合唯一。

软删除文章只写 `posts.deleted_at`，不删除 `assets` 行，也不调用存储删除。

从草稿变为 `published` 时检查 `post.publish`，并在 `scope=own` 时要求 `author_id` 为当前用户。`post.update` 不能代替发布。

### assets

| 字段 | 说明 |
| --- | --- |
| storage | `qiniu`、`s3` 或 `local` |
| bucket | 空间或桶。local 可空 |
| object_key | 对象键 |
| mime | 媒体类型 |
| size | 字节 |
| owner_id | 上传者。`own` 对照此字段 |
| visibility | `public` 或 `private` |
| checksum | 校验和 |
| deleted_at | 元数据软删除 |

第一版页面只上传公开图片（jpeg、png、gif、webp）。私有签名 URL 留在存储接口上。

### site_settings

与 `profiles` 分开。单行两列业务字段：`site_name`、`tagline`（一句话介绍）。不存放个人简介和社交链接。

### comments（预留）

`post_id`、可空 `user_id`、可空 `parent_id`、正文、`status`（`pending` / `approved` / `spam`）、`deleted_at`。

### friend_links（预留）

名称、URL、说明、`status`（`pending` / `approved` / `hidden`）、`sort_order`。

### view_daily（预留）

`post_id`、`view_date`、`view_count`、`visitor_hashes`（`jsonb`，当日独立访客哈希）。主键 `(post_id, view_date)`。

哈希预定为 HMAC-SHA256，密钥 `VISITOR_HASH_KEY`，输入用 IP 与 User-Agent，原文不入库。不建请求日志表。第一版不写计数。

### profiles（预留）

与 `users` 一对一：`user_id` 唯一、简介、头像（可指向 `assets`）、社交链接（`jsonb`）。

### user_identities

| 字段 | 说明 |
| --- | --- |
| user_id | 所属用户 |
| provider | `password`、`wechat`、`qq`、`dingtalk`、`feishu` |
| app_id | 应用标识。本地密码固定为 `local` |
| provider_user_id | 提供方用户 id。密码登录使用邮箱 |
| union_id | 可空。`provider=wechat` 时必须有值 |
| access_token、refresh_token、token_expires_at | 可选 |
| raw | `jsonb`，提供方原始资料 |

唯一约束 `(provider, app_id, provider_user_id)`。

微信必须留下 `union_id`，不能只存 openid。同一开放平台下多个应用用 `union_id` 识别同一个人。

三方登录新建用户时默认角色为 `reader`。

Provider 接口（已经在 `identity`，不注册 HTTP 路由）：

```text
AuthorizationURL(state) (url, error)
Exchange(ctx, code) (Identity, error)
```

`Identity` 包含 `provider_user_id`、`union_id`、可选 token、`raw`。微信实现必须填写 `union_id`。

### 软删除

文章、评论、用户使用 `deleted_at`。查询和对应的唯一约束都排除已删除行。`assets` 也有 `deleted_at`；删除文章不连带删对象。

## 权限如何生效

需要授权的接口声明一个权限码。中间件确认会话里有这个码，否则拒绝。

`scope=own` 时，服务层比较资源上的 `author_id` 或 `owner_id` 与当前用户。列表接口在查询条件里限定本人，单条接口再次核对，避免只靠前端隐藏。

匿名读者走公开查询，不进入这套权限码。

## 边界

### 现在做

- 读者：首页列表、文章页、分类、标签、归档、标题与摘要搜索、可空的关于页。
- 后台：登录、文章草稿 / 发布 / 编辑 / 软删除、分类、标签、站名和一句话介绍、公开图片直传。
- RBAC 全链路。种子一名 super_admin，以及全部内置角色和权限码。
- PostgreSQL、sqlc、golang-migrate、chi、`/api/v1`。
- 存储接口和 qiniu、s3、local。密钥在环境变量。公开图走 CDN。浏览器直传，API 不转发文件流。
- Nginx 或 Go 托管静态文件，一个 Go API，PostgreSQL。Markdown 在浏览器渲染。
- 表：用户、角色、权限、角色权限、用户角色、菜单、会话、文章、分类、标签、文章标签、资产、站点设置，以及密码这一行 `user_identities`。

### 现在只预留

- 评论：表、`comment.moderate`、`comment.create`、`comment` 包。无页面。
- 友链：表、`link.manage`、`link` 包。无页面。
- 统计：`view_count`、`view_daily`、`stats.read`、`stats` 包。不计数，无统计页，无原始访问日志。
- 个人简介：`profiles`、`profile.update`。关于页先空，无编辑页。
- 三方登录：表、Provider 接口、微信 `union_id`、默认角色 reader。授权与回调路由不注册。
- 私有文件：接口上的短时签名 URL，以及浏览器预览（pdf.js、SheetJS、mammoth）。第一版页面只做公开图片。
- 站内小工具：`/tools` 懒加载占位说明，权限码 `tool.use`。无独立后端，无工具逻辑。
- 预留功能的菜单项先不插入。

### 明确不做

- Redis、MongoDB、Elasticsearch、消息队列。
- 服务端 Office 转换，以及 LibreOffice。
- 部门树、组织架构。
- Node SSR，以及为页面渲染常驻的 Node 进程。
- 让文章或身份接口转发文件字节流。
- 在第一版做评论、友链、统计、简介编辑、三方登录页面和小工具逻辑。

## 下一阶段

第一版范围已经做完。后面才做这些，现在仍然没有页面和业务路由：

1. 评论流程。
2. 友链流程。
3. 阅读计数（`view_count` 与 `view_daily`）。
4. 简介编辑，以及关于页改读 `profiles`。
5. 三方登录的授权和回调路由。新用户默认角色仍是 `reader`，微信必须带 `union_id`。
6. 私有文件的签发路由和浏览器预览。
7. `/tools` 里的具体工具。

超级管理员继续在进程启动时种子，不放进 SQL。私有签名 URL 继续先走 `storage.Service.SignedURL`。

## 相关文档

- 变更：[CHANGES.md](CHANGES.md)
- 交接：[HANDOFF.md](HANDOFF.md)
- 教程：[Tutorials/README.md](Tutorials/README.md)
