# RBAC

口径：[PROJECT_STATE.md](../PROJECT_STATE.md)。用户、会话和菜单属于 `identity`。

## 职责

服务端决定每一个管理接口是否允许执行。前端只根据下发的菜单和权限隐藏按钮；藏得不干净时，接口仍然拒绝。

范围只有两种：`any` 表示全部资源，`own` 表示只限本人。`own` 在服务层比较 `posts.author_id` 或 `assets.owner_id`，不放在前端。

公开读者接口不使用这套权限码。`post.read` 是后台查看文章（含草稿）的码，不是“允许看已发布博客”的码。

没有部门，没有组织树。用户直接挂角色。

## 相关表

- `users`：邮箱、密码哈希（argon2id）、显示名、状态（`active` / `disabled`）、`authz_version`、`deleted_at`。
- `roles`：内置角色 `super_admin`、`admin`、`author`、`reader`。`builtin` 为真的行不能删除。
- `permissions`：权限码。
- `role_permissions`：角色、权限码、`scope`（`any` 或 `own`）。
- `user_roles`：用户与角色多对多。权限取并集；同一码既有 `any` 又有 `own` 时按 `any`。
- `menus`：名称、路由、父级、权限码、排序。
- `sessions`：用户、令牌哈希、过期时间、装载权限时看到的 `authz_version`。

`post.*` 拆成 `post.create`、`post.read`、`post.update`、`post.delete`、`post.publish`。发布是单独的码，有 `post.update` 也不能发布。

另外至少有：`category.manage`、`tag.manage`、`comment.moderate`、`link.manage`、`asset.upload`、`asset.read`、`asset.delete`、`user.manage`、`role.manage`、`setting.manage`、`stats.read`、`tool.use`。

预留补充码：`profile.update`、`comment.create`。第一版没有对应页面。

内置角色：

| 角色 | 授予 | 范围 |
| --- | --- | --- |
| super_admin | 全部权限码 | 全部为 any |
| admin | 文章五个码、分类、标签、评论审核、友链、文件三个码、站点设置 | any。不含 `user.manage`、`role.manage`、`stats.read`、`tool.use` |
| author | 文章的创建、查看、修改、删除，以及文件三个码 | own。默认不含 `post.publish`；如果另授，范围仍是 own |
| reader | `profile.update`、`comment.create` | own。第一版没有页面 |

第一版只种子一名 `super_admin` 用户来使用。四条内置角色和全部权限码都要入库。至少保留一名未删除的 super_admin。

菜单第一版四项：文章（`post.read`）、分类（`category.manage`）、标签（`tag.manage`）、站点设置（`setting.manage`）。预留模块先不插菜单，避免指向不存在的页面。菜单是否显示只看用户有没有该权限码，不看 `own` / `any`。

## 第一版做不做

做：登录、退出、会话、权限中间件、`own` 的服务层检查、按权限下发菜单、种子超管和内置角色。

不做：用户管理页面、角色管理页面、评论审核、友链、统计、简介编辑、小工具。这些权限码先入库，接口和页面后做。

调整某用户的角色，或修改其拥有角色的 `role_permissions` 时，把受影响用户的 `authz_version` 加 1。旧会话下次请求发现版本落后，重新装载权限，不强制登出。

## 主流程：超管发布自己的草稿

1. 浏览器向 `POST /api/v1/auth/login` 提交邮箱和密码。用户须为 `active` 且未软删除。
2. API 校验 argon2id 哈希，查出角色与权限，在 `sessions` 写入令牌哈希和当前 `authz_version`，设置 HttpOnly Cookie。生产环境加 Secure，SameSite=Lax。
3. 前端请求 `GET /api/v1/admin/menus`。超管拥有全部权限码，因此四项第一版菜单都在。
4. 创建草稿走 `post.create`。新行的 `author_id` 设为当前用户，`status=draft`。
5. 保存正文走 `post.update`。若范围是 `own` 且 `author_id` 不是当前用户，服务层拒绝。超管的范围是 `any`，可以改任何人的稿。
6. 发布走另一个接口，声明的权限码是 `post.publish`，不是 `post.update`。通过后把 `status` 改为 `published` 并写入 `published_at`。
7. 若之后有人改了超管的角色，`users.authz_version` 加 1。下一次请求对比会话中的版本，重新读取 `user_roles` 和 `role_permissions`，再决定菜单和接口。

author 在被单独授予 `post.publish` 之前，步骤 6 会被拒绝，即使稿子是本人的。
