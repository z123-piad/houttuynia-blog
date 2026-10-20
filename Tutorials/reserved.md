# 预留模块

口径：[PROJECT_STATE.md](../PROJECT_STATE.md)。

这些模块第一版不做页面、不注册业务路由。表、权限码，以及对应的 Go 包或接口已经留下。站内小工具没有独立后端，只在前端路由上占位。

`site_settings` 不是预留项。站名和一句话介绍属于第一版，放在 `site` 包，与下面的个人简介分开。

## 评论

### 职责

读者以后可在文章下留言，主人审核后才公开。支持回复。第一版文章页不显示评论框。

### 相关表

`comments`，包名 `comment`。

- `post_id`
- `user_id` 可空，留给尚未登录的访客
- `parent_id` 可空，指向另一条评论
- 正文
- `status`：`pending`、`approved`、`spam`
- `deleted_at`：软删除

权限码：`comment.create`（reader 为 own）、`comment.moderate`（admin 与 super_admin 为 any）。

### 第一版做不做

不做页面、不做审核接口。表、两个权限码和 `comment` 包留下。菜单先不插入。

### 主流程（预留）

1. 登录用户提交评论，`user_id` 为本人，`status=pending`。要求 `comment.create`。
2. 持有 `comment.moderate` 的用户把它改为 `approved` 或 `spam`。
3. 文章页只列出 `approved` 且未软删除的评论。
4. 删除填写 `deleted_at`，不物理删除，以便保留回复链。

## 友链

### 职责

保存友情链接，并按状态和排序展示。第一版前台和后台都没有友链页。

### 相关表

`friend_links`，包名 `link`。

- 名称、URL、说明
- `status`：`pending`、`approved`、`hidden`
- `sort_order`

权限码：`link.manage`（admin 与 super_admin）。

### 第一版做不做

不做页面。表、权限码和 `link` 包留下。

### 主流程（预留）

1. 新链接以 `pending` 入库。
2. 管理人把它改为 `approved` 或 `hidden`，并设置 `sort_order`。要求 `link.manage`。
3. 前台只展示 `approved`，按 `sort_order` 排列。

## 统计

### 职责

记录文章被看过多少次，以及某一天有多少次、有哪些独立访客。不保存原始访问日志，没有 IP、User-Agent 或请求行的日志表。

### 相关表

- `posts.view_count`：累计次数。第一版该列存在，值不增加。
- `view_daily`：`post_id`、`view_date`、`view_count`、`visitor_hashes`（`jsonb`）。主键是 `(post_id, view_date)`。

独立访客哈希预定为 HMAC-SHA256。密钥是环境变量 `VISITOR_HASH_KEY`。输入用 IP 与 User-Agent，数据库里只留哈希。包名 `stats`。权限码 `stats.read` 只默认给 super_admin。

### 第一版做不做

不做计数，不做统计页，不配置也可以启动。表、列、权限码和包留下。

### 主流程（预留）

1. 匿名读者成功打开一篇已发布文章。
2. 用 `VISITOR_HASH_KEY` 计算哈希，不把 IP 写入数据库。
3. 当天的 `view_daily` 行若不存在则创建。哈希不在 `visitor_hashes` 里时追加，并把当日 `view_count` 和 `posts.view_count` 加 1。
4. 持有 `stats.read` 的用户以后读取这些汇总。没有逐条请求可查。

## 个人简介

### 职责

存放某个用户的简介、头像和社交链接。关于页以后读它。它不是站点设置：`site_settings` 只有站名和一句话介绍。

### 相关表

`profiles`，与 `users` 一对一，放在 `identity`（它是用户资料，不是整站设置）。

- `user_id` 唯一
- 简介
- 头像，可指向 `assets`
- 社交链接，`jsonb`

权限码：`profile.update`。reader 为 own，super_admin 为 any。

### 第一版做不做

关于页路由可以存在，内容为空，不查询 `profiles`。不做简介编辑页。表和权限码留下。

### 主流程（预留）

1. 用户打开自己的简介编辑，提交简介、头像资产和社交链接。
2. 服务层要求 `profile.update`。范围是 `own` 时，`profiles.user_id` 必须是当前用户。
3. 关于页改为读取要展示的那条 `profiles`，站名和一句话介绍仍读 `site_settings`。

## 站内 TypeScript 小工具

### 职责

以后在同一个 React 应用里放小工具，例如单页计算器或转换器。它们是前端页面，没有单独的后端服务，也不新建数据库。某个工具若要保存数据，在 PostgreSQL 加表，而不是加 Redis 或其他库。

### 相关表

第一版没有工具表。权限码 `tool.use` 先入库，默认只给 super_admin。

### 第一版做不做

不做工具逻辑，不做工具菜单。`/tools` 已经是懒加载空路由，空路由里没有工具实现。

占位约定：

- 路径前缀 `/tools`。
- 路由使用 React 懒加载，单独的 chunk，读者首页不下载工具代码。
- 空位不调用 `/api/v1`。以后若要鉴权，接口声明 `tool.use`，数据进 PostgreSQL。

### 主流程（预留）

1. 用户打开 `/tools/<name>`。路由懒加载对应的 TypeScript 页面。
2. 纯前端工具在浏览器里算完即结束。
3. 需要保存时，再为该工具增加 PostgreSQL 表和 `/api/v1` 接口，并检查 `tool.use`。不为此启动新的进程或新的数据库。
