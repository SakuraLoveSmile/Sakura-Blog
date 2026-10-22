import { http } from "../transport";
import type { StellarContentMetadata, WikiDocPayload } from "./contracts";

// --- Wiki 项目与文档（Spec 35a） ---
export interface WikiHeroPreview {
  type?: string;
  commands?: { codes?: string[] }[];
  image?: string;
}

export interface WikiHeroAnimation {
  type?: string;
  params?: Record<string, unknown>;
}

export interface WikiHeroAction {
  title?: string;
  url?: string;
  icon?: string;
}

export interface WikiHero {
  background?: string;
  animation?: WikiHeroAnimation;
  title?: string;
  description?: string;
  start?: string;
  preview?: WikiHeroPreview;
  actions?: WikiHeroAction[];
}

export interface WikiProject extends StellarContentMetadata {
  id: string;
  name: string;
  title: string;
  headline?: string;
  subtitle?: string;
  description?: string;
  icon?: string;
  cover?: string;
  available?: string;
  url?: string;
  repo?: string;
  branch?: string;
  sort: number;
  pin: boolean;
  visible: boolean;
  hero?: WikiHero;
  tags: string[];
  doc_count?: number;
  published_doc_count?: number;
  has_docs?: boolean;
}

// 文档字段：slug / title / order / parent(父文档 ID, 0=顶级) / content_md / status(published|draft)。
export const wikiApi = {
  listProjects: () => http.get<{ items: WikiProject[] }>("/wiki/projects"),
  getProject: (id: string) => http.get<WikiProject>(`/wiki/projects/${encodeURIComponent(id)}`),
  createProject: (data: Partial<WikiProject> & { id: string }) => http.post<WikiProject>("/wiki/projects", data),
  updateProject: (id: string, data: Partial<WikiProject>) =>
    http.put<WikiProject>(`/wiki/projects/${encodeURIComponent(id)}`, data),
  deleteProject: (id: string) => http.delete(`/wiki/projects/${encodeURIComponent(id)}`),
  listDocs: (project: string) => http.get(`/wiki/${encodeURIComponent(project)}/docs`),
  getDoc: (project: string, slug: string) =>
    http.get(`/wiki/${encodeURIComponent(project)}/docs/${encodeURIComponent(slug)}`),
  createDoc: (project: string, data: WikiDocPayload) => http.post(`/wiki/${encodeURIComponent(project)}/docs`, data),
  updateDoc: (project: string, slug: string, data: WikiDocPayload) =>
    http.put(`/wiki/${encodeURIComponent(project)}/docs/${encodeURIComponent(slug)}`, data),
  deleteDoc: (project: string, slug: string) =>
    http.delete(`/wiki/${encodeURIComponent(project)}/docs/${encodeURIComponent(slug)}`),
};
