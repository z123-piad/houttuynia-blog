import { FormEvent, useEffect, useState } from "react";
import { Link, useParams, useSearchParams } from "react-router-dom";
import ReactMarkdown from "react-markdown";
import { api, formatDate, PublicPost, Site } from "./api";

export function useSite() {
  const [site, setSite] = useState<Site>({ site_name: "houttuynia", tagline: "" });
  useEffect(() => {
    api<Site>("/api/v1/site").then(setSite).catch(() => undefined);
  }, []);
  return site;
}

export function Shell({ children }: { children: React.ReactNode }) {
  const site = useSite();
  return (
    <div className="wrap">
      <header className="top">
        <div>
          <Link className="brand" to="/">{site.site_name}</Link>
          {site.tagline ? <p className="tagline">{site.tagline}</p> : null}
        </div>
        <nav>
          <Link to="/archive">归档</Link>
          <Link to="/search">搜索</Link>
          <Link to="/about">关于</Link>
          <Link to="/admin/posts">后台</Link>
        </nav>
      </header>
      {children}
    </div>
  );
}

export function Home() {
  const [posts, setPosts] = useState<PublicPost[] | null>(null);
  const [error, setError] = useState("");
  useEffect(() => {
    api<PublicPost[]>("/api/v1/posts").then(setPosts).catch((err: Error) => setError(err.message));
  }, []);
  return (
    <Shell>
      {error ? <p className="error">{error}</p> : null}
      {posts && posts.length === 0 ? <p>还没有文章</p> : null}
      {posts?.map((post) => (
        <Link className="card" key={post.slug} to={`/posts/${post.slug}`}>
          <h2>{post.title}</h2>
          <p className="meta">{formatDate(post.published_at)} {post.category ? `· ${post.category.name}` : ""}</p>
          <p>{post.excerpt}</p>
        </Link>
      ))}
    </Shell>
  );
}

export function PostPage() {
  const { slug = "" } = useParams();
  const [post, setPost] = useState<PublicPost | null>(null);
  const [error, setError] = useState("");
  useEffect(() => {
    api<PublicPost>(`/api/v1/posts/${slug}`).then(setPost).catch((err: Error) => setError(err.message));
  }, [slug]);
  return (
    <Shell>
      {error ? <p className="error">{error}</p> : null}
      {post ? (
        <article>
          <h1>{post.title}</h1>
          <p className="meta">{formatDate(post.published_at)} · {post.author_name}</p>
          <div className="markdown"><ReactMarkdown>{post.body || ""}</ReactMarkdown></div>
        </article>
      ) : null}
    </Shell>
  );
}

export function CategoryPage() {
  const { slug = "" } = useParams();
  const [title, setTitle] = useState(slug);
  const [posts, setPosts] = useState<PublicPost[]>([]);
  const [error, setError] = useState("");
  useEffect(() => {
    api<{ category: { name: string }; posts: PublicPost[] }>(`/api/v1/categories/${slug}`)
      .then((data) => { setTitle(data.category.name); setPosts(data.posts); })
      .catch((err: Error) => setError(err.message));
  }, [slug]);
  return (
    <Shell>
      <h1>{title}</h1>
      {error ? <p className="error">{error}</p> : null}
      {posts.length === 0 && !error ? <p>还没有文章</p> : null}
      {posts.map((post) => <Link className="card" key={post.slug} to={`/posts/${post.slug}`}><h2>{post.title}</h2></Link>)}
    </Shell>
  );
}

export function TagPage() {
  const { slug = "" } = useParams();
  const [title, setTitle] = useState(slug);
  const [posts, setPosts] = useState<PublicPost[]>([]);
  const [error, setError] = useState("");
  useEffect(() => {
    api<{ tag: { name: string }; posts: PublicPost[] }>(`/api/v1/tags/${slug}`)
      .then((data) => { setTitle(data.tag.name); setPosts(data.posts); })
      .catch((err: Error) => setError(err.message));
  }, [slug]);
  return (
    <Shell>
      <h1>{title}</h1>
      {error ? <p className="error">{error}</p> : null}
      {posts.map((post) => <Link className="card" key={post.slug} to={`/posts/${post.slug}`}><h2>{post.title}</h2></Link>)}
    </Shell>
  );
}

export function ArchivePage() {
  const [months, setMonths] = useState<{ month: string; posts: PublicPost[] }[]>([]);
  useEffect(() => { api<typeof months>("/api/v1/archive").then(setMonths).catch(() => undefined); }, []);
  return (
    <Shell>
      <h1>归档</h1>
      {months.length === 0 ? <p>还没有文章</p> : null}
      {months.map((month) => (
        <section key={month.month}>
          <h2>{month.month}</h2>
          {month.posts.map((post) => <Link className="card" key={post.slug} to={`/posts/${post.slug}`}>{post.title}</Link>)}
        </section>
      ))}
    </Shell>
  );
}

export function SearchPage() {
  const [params, setParams] = useSearchParams();
  const q = params.get("q") || "";
  const [posts, setPosts] = useState<PublicPost[]>([]);
  const [error, setError] = useState("");
  useEffect(() => {
    if (!q) { setPosts([]); return; }
    api<PublicPost[]>(`/api/v1/search?q=${encodeURIComponent(q)}`).then(setPosts).catch((err: Error) => setError(err.message));
  }, [q]);
  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    setParams({ q: String(data.get("q") || "") });
  }
  return (
    <Shell>
      <h1>搜索</h1>
      <form onSubmit={onSubmit} className="row">
        <input name="q" defaultValue={q} aria-label="搜索标题和摘要" placeholder="搜索标题和摘要" />
        <button type="submit">搜索</button>
      </form>
      {error ? <p className="error">{error}</p> : null}
      {q && posts.length === 0 && !error ? <p>没有匹配的文章</p> : null}
      {posts.map((post) => <Link className="card" key={post.slug} to={`/posts/${post.slug}`}><h2>{post.title}</h2><p>{post.excerpt}</p></Link>)}
    </Shell>
  );
}

export function AboutPage() {
  return (
    <Shell>
      <h1>关于</h1>
      <p>这一页还没有介绍</p>
    </Shell>
  );
}

export function NotFound() {
  return <Shell><h1>页面不存在</h1></Shell>;
}
