# 内容：文章、分类、标签

口径：[PROJECT_STATE.md](../PROJECT_STATE.md)。包名 `content`。

## 职责

保存并展示博客正文。一篇文章有一个作者、至多一个分类、任意个标签。正文是 Markdown 原文，由 React 渲染。

读者看到的列表、文章页、分类、标签、归档和搜索都只包含已发布且未软删除的文章。后台才能看到草稿和归档状态。

搜索用 PostgreSQL，范围是标题和摘要，不引入 Elasticsearch。第一版可以用 `ILIKE`。以后若要分词，仍做在 PostgreSQL 里。

## 相关表

`posts`：

| 字段 | 说明 |
| --- | --- |
| author_id | 作者。`own` 对照此字段 |
| category_id | 可空。一篇文章一个分类，不建文章-分类关联表 |
| slug | 未软删除时唯一。读者 URL 使用它 |
| title | 标题，参与搜索 |
| excerpt | 摘要，参与搜索 |
| body_md | Markdown 原文 |
| status | `draft`、`published`、`archived` |
| published_at | 发布时写入，归档按它的月份分组 |
| view_count | 累计阅读次数。列先留下，第一版不增加 |
| deleted_at | 软删除 |

`categories`：名称、slug、说明、排序。`tags`：名称、slug。`post_tags`：文章与标签多对多，`(post_id, tag_id)` 唯一。

删除文章只填写 `deleted_at`。不删除标签行，不删除 `assets`，也不去对象存储里删文件。见 [存储](storage.md)。

## 第一版做不做

做：

- 读者：首页列表、文章页、分类页、标签页、归档、标题和摘要搜索。
- 后台：草稿、编辑、发布、软删除、分类管理、标签管理。
- 权限：`post.create`、`post.read`、`post.update`、`post.delete`、`post.publish`、`category.manage`、`tag.manage`。

不做：

- 评论框、相关文章推荐、全文分词检索。
- 阅读计数。`view_count` 保持 0，直到统计模块真正实现，见 [预留模块](reserved.md)。
- 一篇文章多个分类。

关于页属于站点外壳，第一版可以为空，不读文章表，也不读个人简介。

## 主流程：从草稿到首页

1. 超管在 `/admin/categories` 创建分类，在 `/admin/tags` 创建标签。这两个接口分别要求 `category.manage` 和 `tag.manage`。
2. 在 `/admin/posts/new` 保存草稿：标题、slug、摘要、Markdown、可选分类和标签。`status=draft`，`author_id` 为当前用户，`published_at` 为空。要求 `post.create`。
3. 若正文需要图片，走 [存储](storage.md) 的直传流程，把返回的 CDN URL 写进 Markdown。
4. 再次保存走 `post.update`。作者角色只有 `own`，服务层要求 `author_id` 等于当前用户。超管为 `any`。
5. 发布要求 `post.publish`。author 默认没有这个码。成功后 `status=published`，写入 `published_at`。
6. 匿名的 `GET /api/v1/posts` 列出这篇文章。`GET /api/v1/posts/{slug}` 返回 Markdown。分类页和标签页按 slug 过滤已发布文章。
7. `GET /api/v1/archive` 按 `published_at` 的月份分组，不单独建归档表。
8. `GET /api/v1/search?q=` 在 PostgreSQL 中匹配未删除且已发布行的 `title` 与 `excerpt`。
9. 后台删除要求 `post.delete`，并做同样的 own/any 检查。API 只设置 `deleted_at`。首页查询不再返回它，对象存储里的图片还在。
