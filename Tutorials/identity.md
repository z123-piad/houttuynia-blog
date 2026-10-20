# 身份与预留的三方登录

口径：[PROJECT_STATE.md](../PROJECT_STATE.md)。包名 `identity`。角色和会话的细节在 [RBAC](rbac.md)。

## 职责

证明“是哪个用户”，再把用户交给 RBAC。第一版只有邮箱密码。三方登录的表和 Provider 接口一起留下，HTTP 路由不注册。

本地密码校验 `users.password_hash`（argon2id）。同时在 `user_identities` 写一行 `provider=password`，这样以后查找“这个人有哪些登录方式”不用分两套表。

三方登录新建的用户默认角色是 `reader`。微信必须保存 `union_id`，不能只存 openid。同一开放平台的多个应用靠 `union_id` 认出同一个人。

## 相关表

`users` 的登录字段：邮箱、密码哈希、显示名、状态、`authz_version`、`deleted_at`。`disabled` 或已软删除的用户不能登录。

`user_identities`：

| 字段 | 说明 |
| --- | --- |
| user_id | 所属用户 |
| provider | `password`、`wechat`、`qq`、`dingtalk`、`feishu` |
| app_id | 第三方应用标识。密码登录固定为 `local` |
| provider_user_id | 提供方用户 id。密码登录使用邮箱 |
| union_id | 可空。provider 为 `wechat` 时必须有 |
| access_token、refresh_token、token_expires_at | 可选，没有就空着 |
| raw | `jsonb`，提供方返回的原始资料 |

唯一约束是 `(provider, app_id, provider_user_id)`。

`sessions` 见 RBAC：HttpOnly Cookie，库里只存令牌哈希。

个人简介在 `profiles`，与用户一对一，不属于登录。第一版不编辑简介，见 [预留模块](reserved.md)。

## Provider 接口

接口已经放在 `identity` 包，不在 chi 上注册路由。

```text
AuthorizationURL(state) (url, error)
Exchange(ctx, code) (Identity, error)
```

`Identity` 至少带有 `provider_user_id`、`union_id`、可选 token 和 `raw`。微信的 `Exchange` 必须填 `union_id`，缺了就拒绝写入。QQ、钉钉、飞书在提供方有对应联合标识时写入 `union_id`，没有则留空。

密钥（各平台的 app secret）只放环境变量，不进 `raw` 以外的前端，也不进仓库。`raw` 只存提供方返回的资料。

## 第一版做不做

做：

- `POST /api/v1/auth/login`、`POST /api/v1/auth/logout`、`GET /api/v1/auth/me`。
- 种子超管时写入密码哈希，并插入 `provider=password`、`app_id=local` 的身份行。
- 建好 `user_identities` 的约束，包括“微信行必须有 union_id”。
- Provider 接口的类型定义。

不做：

- 微信、QQ、钉钉、飞书的授权地址和回调路由。不挂 `/api/v1/auth/oauth/...`。
- 三方登录的按钮和页面。
- 用三方资料自动填写 `profiles`。

## 主流程：密码登录

1. 用户打开 `/admin/login`，提交邮箱和密码。
2. API 查找未软删除且 `status=active` 的 `users` 行，校验 argon2id。
3. 确认存在对应的 `user_identities`（`provider=password`）。种子用户在迁移时已经有这一行。
4. 装载角色和权限，写入 `sessions`，设置 HttpOnly Cookie。
5. `GET /api/v1/auth/me` 返回显示名、角色和权限码，供前端隐藏按钮。真正的拒绝仍由各接口的权限中间件完成。
6. 退出删除会话行并清除 Cookie。

## 预留流程：微信登录（路由不注册）

这条流程只说明以后怎么接，第一版没有 URL。

1. 前端向尚未注册的接口索取 `AuthorizationURL(state)`。
2. 浏览器到微信授权，微信带着 `code` 回到回调。
3. `Exchange` 用 code 换身份。实现必须读到 `union_id`。原始 JSON 放入 `raw`，令牌可写入可选的 token 字段。
4. 用 `(provider, app_id, provider_user_id)` 查找 `user_identities`。找到则登录该用户。
5. 找不到时，再按 `provider=wechat` 且 `union_id` 相同查找。找到则是同一开放平台下的另一个应用，绑定到已有用户，而不是再注册一个账号。
6. 仍然找不到则创建 `users`（没有本地密码也可），插入身份行，并授予 `reader`。
7. 会话与密码登录相同：HttpOnly Cookie，权限从角色装载。reader 默认只能在以后编辑本人简介和评论，进不了第一版的文章管理菜单。
