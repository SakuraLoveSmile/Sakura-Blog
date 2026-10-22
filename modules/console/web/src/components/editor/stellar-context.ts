import type { ComputedRef, InjectionKey } from "vue";

export interface StellarEditorMetadata {
  stellar_version: number;
  stellar_config: string;
  blog_extensions: string;
  title?: string;
  layout?: string;
  path?: string;
  language?: string;
}
// 宿主提供整个候选，不解析再编码原始 JSON。服务端从严格站点快照取 theme 和公开 data。
export const stellarEditorContextKey: InjectionKey<ComputedRef<StellarEditorMetadata>> =
  Symbol("stellar-editor-context");
