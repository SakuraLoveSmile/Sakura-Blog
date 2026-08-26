import { http } from "../transport";
import type { RAGAskRequest, RAGAskResponse, RAGStatusResponse } from "../generated";

export const ragApi = {
  status: () => http.get<RAGStatusResponse>("/rag/status"),
  rebuild: () => http.post<RAGStatusResponse>("/rag/rebuild"),
  ask: (question: string) => http.post<RAGAskResponse>("/rag/ask", { question } satisfies RAGAskRequest),
};
