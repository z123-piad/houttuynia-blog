# 变更记录

记录每次改动和验证结果。新的记录写在最上面。

## 2026-10-12 实现第一版 0.1.0

- 改动：读者页、后台、Go API（chi、`/api/v1`、sqlc、golang-migrate）、RBAC（一名可用的 super_admin）、本地存储，以及评论、友链、统计、简介、三方登录的预留表和包。固定文档更新到 0.1.0。
- 未做：评论、友链、阅读计数、简介编辑、三方登录路由、私有文件签发路由、小工具逻辑。`/tools` 只有懒加载空位。
- 验证：迁移在本机 PostgreSQL 上 up / down / up。`go test ./...` 覆盖登录、发布、公开列表、搜索不匹配正文、归档、分类、标签、软删除、作者不能改他人文章、作者不能发布、权限版本重载后不登出、local 上传、七牛和 S3 缺密钥时拒绝。`cd web && npm run build` 通过，`/tools` 是单独的 chunk。本环境没有 Docker，没有跑 `docker compose up`。

## 2026-10-12 写入固定文档、代码尚未开始

- 改动：在仓库根目录写入 `PROJECT_STATE.md`、`CHANGES.md`、`HANDOFF.md` 和 `Tutorials/`（索引、总体架构、RBAC、内容、存储、身份与三方登录、预留模块）。`README.md` 保持标题 houttuynia-blog，只增加一行指向这些文档的链接。
- 未做：没有博客功能代码，没有 Go 模块，没有 React 工程，没有数据库迁移，没有安装依赖。
- 验证：文档已落在根目录（`PROJECT_STATE.md`、`CHANGES.md`、`HANDOFF.md`、`Tutorials/`）。没有可跑的测试。
