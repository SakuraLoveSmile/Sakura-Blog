import { http } from "../transport";
import type { UCMeResponse as GeneratedUCMeResponse } from "../generated";

export interface UCPostItem {
  slug: string;
  title: string;
  excerpt: string;
  content_md: string;
  status: string;
  cover: string;
  tags: string;
  authors: string[];
  author: string;
  created_at: string;
  updated_at: string;
  views: number;
  has_draft: boolean;
  draft?: Partial<UCPostPayload>;
}

export interface UCPostPayload {
  slug?: string;
  title: string;
  excerpt?: string;
  content_md: string;
  status?: "draft" | "published";
  tags?: string;
}

export interface UCAttachmentItem {
  id: number;
  filename: string;
  url: string;
  mime: string;
  size: number;
  created_at: string;
  thumb_url: string;
  width: number;
  height: number;
  group_id: number;
}

export interface UCPageParams {
  page?: number;
  size?: number;
  status?: "draft" | "published" | "trashed";
}

export interface UCAttachmentParams {
  page?: number;
  size?: number;
  keyword?: string;
  kind?: "image" | "other";
}

export const ucApi = {
  me: () => http.get<GeneratedUCMeResponse>("/uc/me"),
  posts: {
    list: (params?: UCPageParams) =>
      http.get<{ items: UCPostItem[]; total: number; page: number; size: number }>("/uc/posts", { params }),
    get: (slug: string) => http.get<UCPostItem>(`/uc/posts/${slug}`),
    create: (data: UCPostPayload) => http.post<UCPostItem>("/uc/posts", data),
    update: (slug: string, data: UCPostPayload) => http.put<UCPostItem>(`/uc/posts/${slug}`, data),
    delete: (slug: string) => http.delete(`/uc/posts/${slug}`),
    saveDraft: (slug: string, data: UCPostPayload) => http.put(`/uc/posts/${slug}/draft`, data),
    publish: (slug: string, data: UCPostPayload) => http.post<UCPostItem>(`/uc/posts/${slug}/publish`, data),
    discardDraft: (slug: string) => http.delete(`/uc/posts/${slug}/draft`),
  },
  attachments: {
    list: (params?: UCAttachmentParams) =>
      http.get<{ items: UCAttachmentItem[]; total: number; page: number; size: number }>("/uc/attachments", { params }),
    upload: (file: File) => {
      const form = new FormData();
      form.append("file", file);
      return http.post<UCAttachmentItem>("/uc/attachments/upload", form, {
        headers: { "Content-Type": "multipart/form-data" },
      });
    },
    importURL: (url: string) => http.post<UCAttachmentItem>("/uc/attachments/import-url", { url }),
    delete: (id: number) => http.delete(`/uc/attachments/${id}`),
  },
};
