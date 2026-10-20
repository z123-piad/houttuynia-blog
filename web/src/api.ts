export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  if (init.body && !(init.body instanceof FormData) && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  const response = await fetch(path, { ...init, headers, credentials: "include" });
  if (response.status === 204) {
    return undefined as T;
  }
  const text = await response.text();
  const data = text ? JSON.parse(text) : null;
  if (!response.ok) {
    throw new ApiError(response.status, data?.error || "请求失败");
  }
  return data as T;
}

export type Site = { site_name: string; tagline: string };
export type Ref = { id?: number; name: string; slug: string };
export type PublicPost = {
  slug: string;
  title: string;
  excerpt: string;
  body?: string;
  published_at: string | null;
  author_name: string;
  category?: Ref | null;
  tags: Ref[];
};
export type AdminPost = {
  id: number;
  title: string;
  slug: string;
  excerpt?: string;
  body?: string;
  status: string;
  category_id?: number | null;
  tag_ids?: number[];
  updated_at: string;
  published_at: string | null;
};
export type Category = { id: number; name: string; slug: string; description: string; sort_order: number };
export type Tag = { id: number; name: string; slug: string };
export type Menu = { name: string; route: string };
export type Me = { id: number; email: string; display_name: string; roles: string[] };

export function formatDate(value: string | null | undefined) {
  if (!value) return "";
  return new Intl.DateTimeFormat("zh-CN", { dateStyle: "medium", timeZone: "UTC" }).format(new Date(value));
}

export async function sha256Hex(file: ArrayBuffer) {
  const digest = await crypto.subtle.digest("SHA-256", file);
  return [...new Uint8Array(digest)].map((b) => b.toString(16).padStart(2, "0")).join("");
}
