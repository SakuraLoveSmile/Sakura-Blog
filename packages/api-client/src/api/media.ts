import { http } from "../transport";

// --- Attachments ---
export const attachmentApi = {
  list: (params?: Record<string, unknown>) => http.get("/attachments", { params }),
  upload: (file: File, groupId?: number) => {
    const form = new FormData();
    form.append("file", file);
    if (groupId != null && groupId > 0) form.append("group_id", String(groupId));
    return http.post("/attachments/upload", form, {
      headers: { "Content-Type": "multipart/form-data" },
    });
  },
  importURL: (url: string, groupId?: number) => http.post("/attachments/import-url", { url, group_id: groupId || 0 }),
  refs: (id: number) => http.get(`/attachments/${id}/refs`),
  delete: (id: number) => http.delete(`/attachments/${id}`),
  move: (ids: number[], groupId: number) => http.post("/attachments/move", { ids, group_id: groupId }),
};

// --- Attachment Groups（Spec 44） ---
export const attachmentGroupApi = {
  list: () => http.get("/attachment-groups"),
  create: (data: { name: string; order?: number }) => http.post("/attachment-groups", data),
  update: (id: number, data: { name: string; order?: number }) => http.put(`/attachment-groups/${id}`, data),
  delete: (id: number) => http.delete(`/attachment-groups/${id}`),
};

// --- Assets (素材速查：插件库图标画廊数据源) ---
export interface AssetIconItem {
  key: string;
  /** 内联 SVG 或图片 URL */
  value: string;
}

export const assetsApi = {
  icons: () => http.get<{ items: AssetIconItem[] }>("/assets/icons"),
};
