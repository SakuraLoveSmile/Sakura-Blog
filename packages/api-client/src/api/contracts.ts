/** JSON values used by deliberately schema-driven settings/preview surfaces. */
export type JsonPrimitive = string | number | boolean | null;
export type JsonValue = JsonPrimitive | JsonObject | JsonValue[];
// Settings and preview payloads are intentionally open schemas. Keep the
// dynamic boundary explicit instead of leaking Record<string, unknown> into
// every domain API.
export interface JsonObject {
  [key: string]: unknown;
}

export interface PageParams {
  page?: number;
  size?: number;
}

export type ContentStatus = "draft" | "published" | "trashed";

export interface PostListParams extends PageParams {
  status?: ContentStatus | string;
  category?: string;
  tag?: string;
  search?: string;
}

export interface PostPoster {
  headline?: string;
  topic?: string;
  caption?: string;
  color?: string;
}

export interface PostPayload {
  slug?: string;
  title?: string;
  excerpt?: string;
  banner?: string;
  cover?: string;
  category?: string;
  categories?: string[];
  category_ids?: number[];
  topic_id?: number | string | null;
  topic?: string;
  tags?: string;
  content_md?: string;
  status?: Exclude<ContentStatus, "trashed"> | string;
  pinned?: boolean;
  password?: string;
  publish_at?: string;
  poster?: PostPoster | null;
  meta_description?: string;
  meta_keywords?: string;
  ai_label?: string;
  authors?: string[];
  author?: string;
}

export interface CommentListParams extends PageParams {
  status?: string;
  keyword?: string;
  user_id?: number;
}

export interface PageListParams extends PageParams {
  status?: ContentStatus | string;
  keyword?: string;
}

export interface PageWidget {
  name: string;
  params?: JsonObject;
}

export interface PageBannerInfo {
  title?: string;
  subtitle?: string;
  avatar?: string;
  icon?: string;
}

export interface PagePayload {
  slug?: string;
  title?: string;
  content_md?: string;
  status?: Exclude<ContentStatus, "trashed"> | string;
  banner?: string;
  banner_info?: PageBannerInfo;
  breadcrumb?: boolean;
  author?: string;
  date?: string;
  leftbar_widgets?: PageWidget[] | null;
  rightbar_widgets?: PageWidget[] | null;
}

export type MomentListParams = PageParams;

export interface MomentPayload {
  content_md?: string;
  images?: string[];
  visibility?: string;
}

export interface ShareListParams extends PageParams {
  post_slug?: string;
}

export interface AttachmentListParams extends PageParams {
  keyword?: string;
  kind?: "image" | "other";
  group?: number;
}

export interface LinkPayload {
  id?: number;
  group_title?: string;
  name?: string;
  url?: string;
  avatar?: string;
  description?: string;
  rss?: string;
  label?: string;
  color?: string;
  sort?: number;
  snapshot?: string;
  last_status?: number;
  last_checked_at?: number;
}

export interface WikiDocPayload {
  slug?: string;
  title?: string;
  order?: number;
  parent?: number;
  content_md?: string;
  status?: Exclude<ContentStatus, "trashed"> | string;
}

export interface SeoSettingsPayload {
  push?: {
    enabled?: boolean;
    indexnow_key?: string;
    baidu_token?: string;
    engines?: string[];
  };
  crawler?: { enabled?: boolean };
  traffic?: { enabled?: boolean };
}
