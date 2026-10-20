import { FormEvent, useEffect, useState } from "react";
import { Link, Navigate, useLocation, useNavigate, useParams } from "react-router-dom";
import { api, ApiError, Category, formatDate, Me, Menu, sha256Hex, Site, Tag, AdminPost } from "./api";

function useMe() {
  const [me, setMe] = useState<Me | null | undefined>(undefined);
  useEffect(() => {
    api<Me>("/api/v1/auth/me").then(setMe).catch(() => setMe(null));
  }, []);
  return me;
}

export function LoginPage() {
  const navigate = useNavigate();
  const [error, setError] = useState("");
  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    try {
      await api("/api/v1/auth/login", {
        method: "POST",
        body: JSON.stringify({ email: data.get("email"), password: data.get("password") }),
      });
      navigate("/admin/posts");
    } catch (err) {
      setError(err instanceof Error ? err.message : "登录失败");
    }
  }
  return (
    <main className="wrap">
      <h1>登录</h1>
      <form onSubmit={onSubmit}>
        <label htmlFor="email">邮箱</label>
        <input id="email" name="email" type="email" autoComplete="username" required />
        <label htmlFor="password">密码</label>
        <input id="password" name="password" type="password" autoComplete="current-password" required />
        {error ? <p className="error">{error}</p> : null}
        <p><button type="submit">登录</button></p>
      </form>
    </main>
  );
}

function AdminFrame({ children }: { children: React.ReactNode }) {
  const me = useMe();
  const [menus, setMenus] = useState<Menu[]>([]);
  const location = useLocation();
  const notice = (location.state as { notice?: string } | null)?.notice;
  useEffect(() => {
    if (me) api<Menu[]>("/api/v1/admin/menus").then(setMenus).catch(() => undefined);
  }, [me]);
  if (me === undefined) return <p className="wrap">加载中</p>;
  if (me === null) return <Navigate to="/admin/login" replace />;
  async function logout() {
    await api("/api/v1/auth/logout", { method: "POST" });
    window.location.assign("/admin/login");
  }
  return (
    <div className="admin">
      <aside className="side">
        <strong>{me.display_name}</strong>
        {menus.map((menu) => <Link key={menu.route} to={menu.route}>{menu.name}</Link>)}
        <button className="ghost" type="button" onClick={logout}>退出</button>
      </aside>
      <main className="main">
        {notice ? <p className="notice">{notice}</p> : null}
        {children}
      </main>
    </div>
  );
}

const statusText: Record<string, string> = { draft: "草稿", published: "已发布", archived: "已归档" };

export function PostList() {
  const [posts, setPosts] = useState<AdminPost[]>([]);
  const [error, setError] = useState("");
  useEffect(() => {
    api<AdminPost[]>("/api/v1/admin/posts").then(setPosts).catch((err: Error) => setError(err.message));
  }, []);
  return (
    <AdminFrame>
      <div className="row"><h1>文章</h1><Link to="/admin/posts/new">新建</Link></div>
      {error ? <p className="error">{error}</p> : null}
      {posts.length === 0 ? <p>还没有文章</p> : null}
      <table>
        <tbody>
          {posts.map((post) => (
            <tr key={post.id}>
              <td><Link to={`/admin/posts/${post.id}/edit`}>{post.title}</Link></td>
              <td>{statusText[post.status] || post.status}</td>
              <td>{formatDate(post.updated_at)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </AdminFrame>
  );
}

export function PostEditor() {
  const { id } = useParams();
  const navigate = useNavigate();
  const [categories, setCategories] = useState<Category[]>([]);
  const [tags, setTags] = useState<Tag[]>([]);
  const [title, setTitle] = useState("");
  const [slug, setSlug] = useState("");
  const [excerpt, setExcerpt] = useState("");
  const [body, setBody] = useState("");
  const [categoryID, setCategoryID] = useState("");
  const [tagIDs, setTagIDs] = useState<number[]>([]);
  const [error, setError] = useState("");

  useEffect(() => {
    api<Category[]>("/api/v1/categories").then(setCategories).catch(() => undefined);
    api<Tag[]>("/api/v1/tags").then(setTags).catch(() => undefined);
    if (!id) return;
    api<AdminPost>(`/api/v1/admin/posts/${id}`).then((post) => {
      setTitle(post.title);
      setSlug(post.slug);
      setExcerpt(post.excerpt || "");
      setBody(post.body || "");
      setCategoryID(post.category_id ? String(post.category_id) : "");
      setTagIDs(post.tag_ids || []);
    }).catch((err: Error) => setError(err.message));
  }, [id]);

  function payload() {
    return {
      title, slug, excerpt, body,
      category_id: categoryID ? Number(categoryID) : null,
      tag_ids: tagIDs,
    };
  }

  async function save() {
    setError("");
    try {
      if (id) {
        await api(`/api/v1/admin/posts/${id}`, { method: "PUT", body: JSON.stringify(payload()) });
        navigate(`/admin/posts/${id}/edit`, { state: { notice: "已保存草稿" } });
      } else {
        const created = await api<AdminPost>("/api/v1/admin/posts", { method: "POST", body: JSON.stringify(payload()) });
        navigate(`/admin/posts/${created.id}/edit`, { state: { notice: "已保存草稿" } });
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "保存失败");
    }
  }

  async function publish() {
    if (!id) {
      setError("请先保存草稿");
      return;
    }
    try {
      await api(`/api/v1/admin/posts/${id}`, { method: "PUT", body: JSON.stringify(payload()) });
      await api(`/api/v1/admin/posts/${id}/publish`, { method: "POST" });
      navigate("/admin/posts", { state: { notice: "已发布" } });
    } catch (err) {
      setError(err instanceof Error ? err.message : "发布失败");
    }
  }

  async function remove() {
    if (!id) return;
    await api(`/api/v1/admin/posts/${id}`, { method: "DELETE" });
    navigate("/admin/posts", { state: { notice: "已删除" } });
  }

  async function upload(file: File) {
    const bytes = await file.arrayBuffer();
    const cred = await api<{
      upload_url: string;
      method: string;
      headers?: Record<string, string>;
      object_key: string;
      public_url: string;
      confirm_token: string;
    }>("/api/v1/assets/upload-token", {
      method: "POST",
      body: JSON.stringify({ mime: file.type, size: file.size, visibility: "public" }),
    });
    const put = await fetch(cred.upload_url, { method: cred.method, headers: cred.headers, body: bytes });
    if (!put.ok) throw new ApiError(put.status, "上传失败");
    const checksum = await sha256Hex(bytes);
    const saved = await api<{ public_url: string }>("/api/v1/assets", {
      method: "POST",
      body: JSON.stringify({ object_key: cred.object_key, checksum, confirm_token: cred.confirm_token }),
    });
    setBody((current) => `${current}\n![image](${saved.public_url})\n`);
  }

  return (
    <AdminFrame>
      <h1>{id ? "编辑文章" : "新建文章"}</h1>
      {error ? <p className="error">{error}</p> : null}
      <label htmlFor="title">标题</label>
      <input id="title" value={title} onChange={(e) => setTitle(e.target.value)} />
      <label htmlFor="slug">标识</label>
      <input id="slug" value={slug} onChange={(e) => setSlug(e.target.value)} />
      <label htmlFor="excerpt">摘要</label>
      <textarea id="excerpt" value={excerpt} onChange={(e) => setExcerpt(e.target.value)} />
      <label htmlFor="body">正文</label>
      <textarea id="body" value={body} onChange={(e) => setBody(e.target.value)} />
      <label htmlFor="category">分类</label>
      <select id="category" value={categoryID} onChange={(e) => setCategoryID(e.target.value)}>
        <option value="">不选</option>
        {categories.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
      </select>
      <fieldset>
        <legend>标签</legend>
        {tags.map((tag) => (
          <label key={tag.id}>
            <input type="checkbox" checked={tagIDs.includes(tag.id)} onChange={() => {
              setTagIDs((current) => current.includes(tag.id) ? current.filter((item) => item !== tag.id) : [...current, tag.id]);
            }} /> {tag.name}
          </label>
        ))}
      </fieldset>
      <label htmlFor="image">公开图片</label>
      <input id="image" type="file" accept="image/jpeg,image/png,image/gif,image/webp" onChange={(e) => {
        const file = e.target.files?.[0];
        if (file) upload(file).catch((err: Error) => setError(err.message));
      }} />
      <p className="row">
        <button type="button" onClick={save}>保存草稿</button>
        <button type="button" onClick={publish}>发布</button>
        {id ? <button className="ghost" type="button" onClick={remove}>删除</button> : null}
      </p>
    </AdminFrame>
  );
}

export function CategoriesPage() {
  const [rows, setRows] = useState<Category[]>([]);
  const [error, setError] = useState("");
  async function load() { setRows(await api<Category[]>("/api/v1/categories")); }
  useEffect(() => { load().catch((err: Error) => setError(err.message)); }, []);
  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    try {
      await api("/api/v1/admin/categories", {
        method: "POST",
        body: JSON.stringify({ name: data.get("name"), slug: data.get("slug"), description: data.get("description") || "", sort_order: Number(data.get("sort_order") || 0) }),
      });
      event.currentTarget.reset();
      await load();
    } catch (err) { setError(err instanceof Error ? err.message : "保存失败"); }
  }
  return (
    <AdminFrame>
      <h1>分类</h1>
      {error ? <p className="error">{error}</p> : null}
      <form onSubmit={onSubmit}>
        <label htmlFor="cname">名称</label><input id="cname" name="name" required />
        <label htmlFor="cslug">标识</label><input id="cslug" name="slug" required />
        <label htmlFor="cdesc">说明</label><input id="cdesc" name="description" />
        <label htmlFor="csort">排序</label><input id="csort" name="sort_order" defaultValue="0" />
        <p><button type="submit">添加</button></p>
      </form>
      <ul>{rows.map((row) => <li key={row.id}>{row.name} / {row.slug} <button className="ghost" type="button" onClick={() => api(`/api/v1/admin/categories/${row.id}`, { method: "DELETE" }).then(load)}>删除</button></li>)}</ul>
    </AdminFrame>
  );
}

export function TagsPage() {
  const [rows, setRows] = useState<Tag[]>([]);
  const [error, setError] = useState("");
  async function load() { setRows(await api<Tag[]>("/api/v1/tags")); }
  useEffect(() => { load().catch((err: Error) => setError(err.message)); }, []);
  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    try {
      await api("/api/v1/admin/tags", { method: "POST", body: JSON.stringify({ name: data.get("name"), slug: data.get("slug") }) });
      event.currentTarget.reset();
      await load();
    } catch (err) { setError(err instanceof Error ? err.message : "保存失败"); }
  }
  return (
    <AdminFrame>
      <h1>标签</h1>
      {error ? <p className="error">{error}</p> : null}
      <form onSubmit={onSubmit}>
        <label htmlFor="tname">名称</label><input id="tname" name="name" required />
        <label htmlFor="tslug">标识</label><input id="tslug" name="slug" required />
        <p><button type="submit">添加</button></p>
      </form>
      <ul>{rows.map((row) => <li key={row.id}>{row.name} / {row.slug} <button className="ghost" type="button" onClick={() => api(`/api/v1/admin/tags/${row.id}`, { method: "DELETE" }).then(load)}>删除</button></li>)}</ul>
    </AdminFrame>
  );
}

export function SettingsPage() {
  const [site, setSite] = useState<Site>({ site_name: "", tagline: "" });
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");
  useEffect(() => { api<Site>("/api/v1/site").then(setSite).catch((err: Error) => setError(err.message)); }, []);
  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    try {
      const saved = await api<Site>("/api/v1/admin/settings", { method: "PUT", body: JSON.stringify(site) });
      setSite(saved);
      setNotice("已保存");
    } catch (err) { setError(err instanceof Error ? err.message : "保存失败"); }
  }
  return (
    <AdminFrame>
      <h1>站点设置</h1>
      {notice ? <p className="notice">{notice}</p> : null}
      {error ? <p className="error">{error}</p> : null}
      <form onSubmit={onSubmit}>
        <label htmlFor="site_name">站名</label>
        <input id="site_name" value={site.site_name} onChange={(e) => setSite({ ...site, site_name: e.target.value })} />
        <label htmlFor="tagline">一句话介绍</label>
        <input id="tagline" value={site.tagline} onChange={(e) => setSite({ ...site, tagline: e.target.value })} />
        <p><button type="submit">保存</button></p>
      </form>
    </AdminFrame>
  );
}
