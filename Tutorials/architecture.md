# 总体架构

口径：[PROJECT_STATE.md](../PROJECT_STATE.md)。版本 `0.1.0`，第一版代码已经落地。

## 职责

系统是一个个人博客：读者看已发布的文章，主人在同一套前端里管理内容和站点设置。

- 前台是 React，用 Vite 打成静态文件。读者页和后台在同一个应用里。后台路由前缀 `/admin`。以后的单页小工具放 `/tools`，路由懒加载。
- 后端是一个 Go 进程，HTTP API 前缀 `/api/v1`。
- 数据只在 PostgreSQL。会话、搜索、第三方原始资料都在这一个库里。
- 文件通过存储接口进出。浏览器直接把公开图片传到七牛（或当前驱动的上传目标），API 不转发文件流。

路由库选定 chi（`github.com/go-chi/chi/v5`）。它基于 `net/http`，路由组适合 `/api/v1`，中间件适合会话和 RBAC，依赖少。标准库可以匹配路径，但分组和中间件要自己搭；chi 只补这一层，不必换更重的 Web 框架。

查询用 sqlc，迁移用 golang-migrate。领域包为 `identity`、`content`、`comment`、`link`、`stats`、`site`、`storage`。模块路径 `github.com/z123-piad/houttuynia-blog`。这些目录已经存在。

Markdown 存在 PostgreSQL，API 把原文交给 React 渲染。小服务器上不跑 Node SSR。

## 相关表

本篇不重复每个字段，字段在 PROJECT_STATE 和分篇里。按包划分：

| 包 | 表 |
| --- | --- |
| identity | `users`、`roles`、`permissions`、`role_permissions`、`user_roles`、`menus`、`sessions`、`user_identities`、`profiles` |
| content | `posts`、`categories`、`tags`、`post_tags` |
| comment | `comments` |
| link | `friend_links` |
| stats | `view_daily`（累计次数在 `posts.view_count`） |
| site | `site_settings` |
| storage | `assets` |

进程之间没有缓存库，也没有消息队列。静态文件由 Nginx 或 Go 发出，API 只负责 JSON 和存储凭证。

## 第一版做不做

做：

- 读者能打开首页、文章、分类、标签、归档、搜索和关于页。关于页可以先没有正文。
- 主人能登录后台，管理文章、分类、标签、站名和一句话介绍，并上传公开图片。
- 权限链路按 RBAC 接通，种子数据只有一名 `super_admin`。

不做页面。表、权限码、包或接口已经留下：评论、友链、统计、个人简介、三方登录、小工具。见 [预留模块](reserved.md) 和 [身份](identity.md)。

明确不引入：Redis、MongoDB、Elasticsearch、消息队列、服务端 Office 转换、LibreOffice、部门树、Node SSR。

## 主流程：读者打开一篇已发布文章

这条路径已经有代码。流程如下。

1. 浏览器请求站点根路径，Nginx 或 Go 返回 Vite 的 `index.html` 和静态资源。
2. React 请求 `GET /api/v1/posts`。这是匿名接口，只查询 `status=published` 且未软删除的行，按 `published_at` 倒序。不检查 `post.read`。
3. 同时请求 `GET /api/v1/site`，拿到 `site_name` 和 `tagline`。这两项来自 `site_settings`，不是个人简介。
4. 读者点开一篇文章，前端进入 `/posts/:slug`，请求 `GET /api/v1/posts/{slug}`。
5. API 再次确认该行已发布且未删除，返回标题、摘要、Markdown、分类、标签和发布时间。
6. React 把 Markdown 渲染成页面。文中的图片地址是上传时登记的 CDN URL，浏览器直接向 CDN 取图，不经过 API。
7. 归档、分类、标签和搜索都是同类公开查询。搜索只匹配标题和摘要。关于页第一版可以返回空正文。

后台请求才会带上 HttpOnly 会话 Cookie，并进入 [RBAC](rbac.md)。
