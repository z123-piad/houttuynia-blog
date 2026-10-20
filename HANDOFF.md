# 交接

## 当前任务

个人博客第一版 `0.1.0` 已写在这个仓库里：读者页、后台、Go API、RBAC、预留表和固定文档。

## 口径

技术选型、表、权限和边界以 [PROJECT_STATE.md](PROJECT_STATE.md) 为准。教程在 [Tutorials/README.md](Tutorials/README.md)。

路由库是 chi（`github.com/go-chi/chi/v5`）。查询用 sqlc，迁移用 golang-migrate。数据库只有 PostgreSQL。

超级管理员在 API 启动时按 `SEED_ADMIN_EMAIL` / `SEED_ADMIN_PASSWORD` 种子。邮箱已存在时不重置密码。私有文件的短时链接在 `storage.Service.SignedURL`，签发用的 HTTP 路由不注册。

## 修改文件

- `cmd/api`、`internal/`、`db/`、`web/`：第一版实现。
- `docker-compose.yml`、`.env.example`、`sqlc.yaml`、`.gitignore`。
- `README.md`：本地运行步骤。标题仍是 houttuynia-blog。
- `PROJECT_STATE.md`、`CHANGES.md`、`HANDOFF.md`、`Tutorials/`：版本 `0.1.0`，并写明哪些已经能用、哪些仍只是表或接口。

## 测试

`go test ./...` 使用 `TEST_DATABASE_URL`，缺省连 `houttuynia_test`，不会清空开发库 `houttuynia`。TestMain 做迁移 up / down / up，再种子管理员。前端用 `cd web && npm run build`。

## 下一步

按 [PROJECT_STATE.md](PROJECT_STATE.md) 的「下一阶段」做评论、友链、阅读计数、简介编辑、三方登录路由、私有文件预览和具体小工具。这些现在不要提前做。
