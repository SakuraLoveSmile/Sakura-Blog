import { http } from "../transport";
import type {
  NotebookRequest,
  NotebookResponse,
  NotebookListResponse,
  NotebookNoteRequest,
  NotebookNoteResponse,
  NotebookNoteListResponse,
  NotebookWriteResponse,
} from "../generated";

export interface NotebookListParams {
  page?: number;
  size?: number;
  q?: string;
}

export interface NotebookNoteListParams extends NotebookListParams {
  status?: "draft" | "published";
  tag?: string;
}

const notebookPath = (id: string) => `/notebooks/${encodeURIComponent(id)}`;
const notePath = (notebook: string, id: number) => `${notebookPath(notebook)}/notes/${id}`;

// 笔记本页面自己在内联提示里展示错误（含“需要先迁移”的说明），所以关闭全局错误 Toast；
// 写入失败时由页面发一条简短 Toast，避免把整段后端原文堆在界面上。
const quiet = { mute: true } as const;

export const notebookApi = {
  list: (params?: NotebookListParams) => http.get<NotebookListResponse>("/notebooks", { params, ...quiet }),
  get: (id: string) => http.get<NotebookResponse>(notebookPath(id), quiet),
  create: (data: NotebookRequest) => http.post<NotebookResponse>("/notebooks", data, quiet),
  update: (id: string, data: NotebookRequest) => http.put<NotebookResponse>(notebookPath(id), data, quiet),
  delete: (id: string) => http.delete<NotebookWriteResponse>(notebookPath(id), quiet),
  listNotes: (notebook: string, params?: NotebookNoteListParams) =>
    http.get<NotebookNoteListResponse>(`${notebookPath(notebook)}/notes`, { params, ...quiet }),
  getNote: (notebook: string, id: number) => http.get<NotebookNoteResponse>(notePath(notebook, id), quiet),
  createNote: (notebook: string, data: NotebookNoteRequest) =>
    http.post<NotebookNoteResponse>(`${notebookPath(notebook)}/notes`, data, quiet),
  updateNote: (notebook: string, id: number, data: NotebookNoteRequest) =>
    http.put<NotebookNoteResponse>(notePath(notebook, id), data, quiet),
  deleteNote: (notebook: string, id: number) => http.delete<NotebookWriteResponse>(notePath(notebook, id), quiet),
  publishNote: (notebook: string, id: number, data: NotebookNoteRequest = {}) =>
    http.post<NotebookNoteResponse>(`${notePath(notebook, id)}/publish`, data, quiet),
  saveNoteDraft: (notebook: string, id: number, data: NotebookNoteRequest = {}) =>
    http.put<NotebookNoteResponse>(`${notePath(notebook, id)}/draft`, data, quiet),
};
