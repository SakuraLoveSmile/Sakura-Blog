export interface RuntimePluginManifest {
  id: string;
  name: string;
  version: string;
  kind?: "script" | "bundle";
  bundle?: string;
  hooks: string[];
  permissions: string[];
  network_allowlist?: string[];
}

export interface RuntimePluginModule {
  default?: {
    id: string;
    meta?: { title: string; description?: string; permissions?: string[] };
    widgets?: unknown[];
    nav?: unknown[];
  };
}

export type BundleImporter = (entry: string) => Promise<RuntimePluginModule>;

export function validateBundleEntry(entry: string): boolean {
  if (!entry || !entry.startsWith("/plugins/")) return false;
  if (entry.startsWith("//") || entry.includes("..")) return false;
  for (const character of entry) {
    if (character.charCodeAt(0) < 32) return false;
  }
  return true;
}

const defaultImporter: BundleImporter = (entry) => import(/* @vite-ignore */ entry) as Promise<RuntimePluginModule>;

export async function loadPluginBundle(
  manifest: RuntimePluginManifest,
  importer: BundleImporter = defaultImporter,
): Promise<NonNullable<RuntimePluginModule["default"]>> {
  if (!manifest.bundle || !validateBundleEntry(manifest.bundle)) {
    throw new Error(`plugin ${manifest.id} has no valid same-origin bundle entry`);
  }
  const loaded = await importer(manifest.bundle);
  if (!loaded.default || loaded.default.id !== manifest.id) {
    throw new Error(`plugin ${manifest.id} bundle identity mismatch`);
  }
  return loaded.default;
}
