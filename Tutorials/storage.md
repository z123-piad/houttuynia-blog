# 存储

口径：[PROJECT_STATE.md](../PROJECT_STATE.md)。包名 `storage`。文章包只保存 CDN 或签名流程返回的地址，不接收文件字节。

## 职责

为上传和读取提供统一接口，具体落到哪个厂商由环境变量 `STORAGE_DRIVER` 决定。

接口四个方法：

- 上传凭证：告诉浏览器向哪里传、带什么令牌或签名、对象键是什么、何时过期。
- 公开 URL：公开对象的 CDN 或等价地址。
- 私有签名 URL：短时有效，调用前必须先通过权限。
- 删除：按 bucket 和 object_key 删除对象。

驱动三个：

- `qiniu`：第一版公开图片的主路径。
- `s3`：同一接口兼容阿里云、腾讯云和 MinIO。使用预签名上传。
- `local`：本机目录 `LOCAL_STORAGE_PATH`。上传端点属于这个驱动，不把文件流塞进文章接口。

密钥只出现在环境变量里：七牛的 Access Key、Secret Key、空间和 CDN 域名；S3 的端点、区域、桶和密钥。前端包和 Git 仓库都不放密钥。

公开图片走 CDN。私有文件不把长期地址写进页面；浏览器预览使用短时链接，PDF 用 pdf.js，表格用 SheetJS，Word 用 mammoth。服务端不转换 Office，不安装 LibreOffice。

## 相关表

`assets`：

| 字段 | 说明 |
| --- | --- |
| storage | `qiniu`、`s3` 或 `local` |
| bucket | 空间或桶。local 可空 |
| object_key | 对象键 |
| mime | 媒体类型 |
| size | 字节数 |
| owner_id | 上传者。权限范围 `own` 对照此字段 |
| visibility | `public` 或 `private` |
| checksum | 校验和 |
| deleted_at | 元数据软删除 |

没有单独的“文章图片”表。Markdown 里记下公开 URL。删除文章不更新 `assets`，也不调用删除对象。对象何时物理删除，走资产自己的删除接口。

## 第一版做不做

做：存储接口、三个驱动的位置、公开图片直传、`assets` 登记、权限码 `asset.upload` / `asset.read` / `asset.delete`。第一版页面接受的图片类型为 jpeg、png、gif、webp，可见性为 `public`。

author 的文件权限范围是 `own`，只能为自己的 `owner_id` 上传、读取和删除。admin 和 super_admin 为 `any`。

预留、第一版没有页面：私有文件的短时链接，以及 pdf.js、SheetJS、mammoth 预览。接口方法要留，预览路由不做。

不做：API 代理转发七牛或 S3 的文件流；把密钥发给浏览器长期保存；删文章时连带删桶里的对象。

## 主流程：文章里插入一张公开图

1. 编辑器请求 `POST /api/v1/assets/upload-token`，提交 mime、大小和 `visibility=public`。中间件要求 `asset.upload`。
2. API 拒绝非图片类型。通过后生成 object_key，调用当前驱动产生上传凭证。七牛返回上传令牌和上传地址；S3 返回预签名 PUT。响应里包含到期时间和上传完成后将使用的公开 URL。请求体里没有文件内容。
3. 浏览器把文件直接传到该地址。Go 的文章接口不接触字节流。
4. 上传成功后，浏览器请求 `POST /api/v1/assets`，提交 object_key、mime、size、checksum、visibility。API 写入 `assets`，`owner_id` 为当前用户，`storage` 为当前驱动。
5. 编辑器把公开 URL 插入 Markdown。读者打开文章时，浏览器向 CDN 取图。
6. 删除这张图要求 `asset.delete`。`own` 时 `owner_id` 必须是当前用户。API 调用驱动的删除，并把 `assets.deleted_at` 写上。已经发表的文章不会因为这张图的元数据还在就被拦住；反过来，删文章也不会触发第 6 步。

私有文件以后的路径是：登记时 `visibility=private`，不返回长期 CDN 地址。预览前请求短时 URL，接口先查 `asset.read` 和 own/any，再调用签名方法。浏览器拿短时链接本地预览。这条 HTTP 路由第一版不挂。
