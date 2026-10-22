import type { WikiProject } from "@blog/api-client";

export function wikiProjectDisplayName(project: Pick<WikiProject, "stellar_version" | "id" | "name" | "title">): string {
  // 严格集合只消费 name；旧记录的 name/title 选择留给迁移审查。
  if (project.stellar_version === 2) return project.name || project.id;
  return project.title || project.name || project.id;
}
