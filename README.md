# houttuynia-blog

个人博客。架构以 [PROJECT_STATE.md](PROJECT_STATE.md) 为准，变更见 [CHANGES.md](CHANGES.md)，交接见 [HANDOFF.md](HANDOFF.md)，说明见 [Tutorials/README.md](Tutorials/README.md)。

第一版 `0.1.0`：读者页和后台在同一个 React 应用里，API 是一个 Go 进程，数据在 PostgreSQL。

## 本地运行

需要 Go 1.26（`golang.org/x/crypto` 的 argon2 要求）、Node.js、PostgreSQL 16。

1. 复制环境变量：`cp .env.example .env`
2. 启动数据库：`docker compose up -d`。只起 PostgreSQL。连接串与 `.env.example` 相同：`postgres://houttuynia:houttuynia@localhost:5432/houttuynia?sslmode=disable`
3. 启动 API：`go run ./cmd/api`。默认 `:8080`。启动时执行迁移，并在邮箱还不存在时种子超级管理员。
4. 启动前端：`cd web && npm install && npm run dev`，打开 `http://127.0.0.1:5173`

后台入口是 `/admin/login`。初始账号写在 `.env.example`：`admin@example.com` / `admin123456`。这个邮箱已经存在时，不会重置密码。

已经装了本机 PostgreSQL 时，可以不跑 Docker，只要 `DATABASE_URL` 指向它。

生产静态文件：`cd web && npm run build`，再把 `WEB_DIR` 设为 `web/dist`，由 Go 托管单页应用。

## 测试

```bash
go test ./...
```

测试库默认是 `houttuynia_test`，可用 `TEST_DATABASE_URL` 覆盖。不要把测试指到开发库。

## 现在能用

读者首页、文章、分类、标签、归档、标题和摘要搜索、空的关于页。后台登录、文章草稿 / 编辑 / 发布 / 软删除、分类、标签、站名和一句话介绍、公开图片。`local` 存储可以真正上传。`qiniu` 和 `s3` 在缺少密钥时会拒绝，不会假装成功。

评论、友链、阅读统计、个人简介、三方登录页面和站内小工具仍只是表或接口。`/tools` 是懒加载空位。
