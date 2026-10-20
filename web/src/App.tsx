import { lazy, Suspense } from "react";
import { Navigate, Route, Routes } from "react-router-dom";
import { AboutPage, ArchivePage, CategoryPage, Home, NotFound, PostPage, SearchPage, TagPage } from "./reader";
import { CategoriesPage, LoginPage, PostEditor, PostList, SettingsPage, TagsPage } from "./admin";

const Tools = lazy(() => import("./tools/placeholder"));

export default function App() {
  return (
    <Routes>
      <Route path="/" element={<Home />} />
      <Route path="/posts/:slug" element={<PostPage />} />
      <Route path="/categories/:slug" element={<CategoryPage />} />
      <Route path="/tags/:slug" element={<TagPage />} />
      <Route path="/archive" element={<ArchivePage />} />
      <Route path="/search" element={<SearchPage />} />
      <Route path="/about" element={<AboutPage />} />
      <Route path="/tools" element={<Suspense fallback={<p className="wrap">加载中</p>}><Tools /></Suspense>} />
      <Route path="/admin/login" element={<LoginPage />} />
      <Route path="/admin" element={<Navigate to="/admin/posts" replace />} />
      <Route path="/admin/posts" element={<PostList />} />
      <Route path="/admin/posts/new" element={<PostEditor />} />
      <Route path="/admin/posts/:id/edit" element={<PostEditor />} />
      <Route path="/admin/categories" element={<CategoriesPage />} />
      <Route path="/admin/tags" element={<TagsPage />} />
      <Route path="/admin/settings" element={<SettingsPage />} />
      <Route path="*" element={<NotFound />} />
    </Routes>
  );
}
