// 从锁定的官方源码生成声明数据；仅开发时使用 Node，生产解析器不执行 JS。
"use strict";
const fs = require("node:fs");
const path = require("node:path");
const crypto = require("node:crypto");
const upstream = path.resolve(process.argv[2] || "");
if (!fs.existsSync(path.join(upstream, "scripts/schema/config-schema.js"))) {
  throw new Error("Provide the exported Stellar 2.0.0 source directory");
}
const load = name => require(path.join(upstream, name));
const { CONFIG_SCHEMA, CONFIG_DEFAULTS } = load("scripts/schema/config-schema");
const { FRONT_MATTER_CONFIG_SCHEMA, COLLECTION_CONFIG_SCHEMA } = load("scripts/schema/content-config-schema");
const { heroEffectDefinitions } = load("scripts/lib/hero-effect-registry");
function ordered(node) {
  if (!node || typeof node !== "object") return node;
  const result = structuredClone(node);
  if (node.properties) {
    result.propertyOrder = Object.keys(node.properties);
    result.properties = Object.fromEntries(Object.entries(node.properties).map(([k, n]) => [k, ordered(n)]));
  }
  if (node.items) result.items = ordered(node.items);
  if (node.additionalProperties) result.additionalProperties = ordered(node.additionalProperties);
  return result;
}
const inputs = ["_config.yml", "scripts/schema/config-schema.js", "scripts/schema/config-rules.js", "scripts/schema/content-config-schema.js", "scripts/schema/content-config-rules.js", "scripts/lib/hero-effect-registry.js", "scripts/lib/share-services.js"];
const artifact = {
  tag: "2.0.0", commit: "9d11978a24fe6610e3a7dbab741cf62634539d05",
  inputHashes: Object.fromEntries(inputs.map(file => [file, crypto.createHash("sha256").update(fs.readFileSync(path.join(upstream, file))).digest("hex")])),
  theme: ordered(CONFIG_SCHEMA), page: ordered(FRONT_MATTER_CONFIG_SCHEMA), collection: ordered(COLLECTION_CONFIG_SCHEMA),
  rawDefaults: CONFIG_DEFAULTS,
  effects: heroEffectDefinitions().map(({ presentation, ...definition }) => definition)
};
fs.writeFileSync(path.join(__dirname, "../schema.json"), JSON.stringify(artifact, null, 2) + "\n");
