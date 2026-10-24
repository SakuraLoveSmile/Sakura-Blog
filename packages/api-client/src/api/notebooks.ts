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

export const notebookApi = {
  list: (params?: NotebookListParams) => http.get<NotebookListResponse>("/notebooks", { params }),
  get: (id: string) => http.get<NotebookResponse>(notebookPath(id)),
  create: (data: NotebookRequest) => http.post<NotebookResponse>("/notebooks", data),
  update: (id: string, data: NotebookRequest) => http.put<NotebookResponse>(notebookPath(id), data),
  delete: (id: string) => http.delete<NotebookWriteResponse>(notebookPath(id)),
  listNotes: (notebook: string, params?: NotebookNoteListParams) =>
    http.get<NotebookNoteListResponse>(`${notebookPath(notebook)}/notes`, { params }),
  getNote: (notebook: string, id: number) => http.get<NotebookNoteResponse>(notePath(notebook, id)),
  createNote: (notebook: string, data: NotebookNoteRequest) =>
    http.post<NotebookNoteResponse>(`${notebookPath(notebook)}/notes`, data),
  updateNote: (notebook: string, id: number, data: NotebookNoteRequest) =>
    http.put<NotebookNoteResponse>(notePath(notebook, id), data),
  deleteNote: (notebook: string, id: number) => http.delete<NotebookWriteResponse>(notePath(notebook, id)),
  publishNote: (notebook: string, id: number, data: NotebookNoteRequest = {}) =>
    http.post<NotebookNoteResponse>(`${notePath(notebook, id)}/publish`, data),
  saveNoteDraft: (notebook: string, id: number, data: NotebookNoteRequest = {}) =>
    http.put<NotebookNoteResponse>(`${notePath(notebook, id)}/draft`, data),
};
