// Generate the local Unicode emoji search index used by the comment Boost picker.
// Source: emojibase-data 15.3.2 (MIT), pinned so builds remain reproducible.
import { mkdirSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const VERSION = '15.3.2';
const BASE_URL = `https://cdn.jsdelivr.net/npm/emojibase-data@${VERSION}`;
const outPath = join(dirname(fileURLToPath(import.meta.url)), '..', 'assets', 'js', 'boost-emoji-data.json');

async function load(locale) {
  const response = await fetch(`${BASE_URL}/${locale}/compact.json`);
  if (!response.ok) throw new Error(`Failed to fetch ${locale} emoji data: ${response.status}`);
  return response.json();
}

function searchText(en, zh) {
  return [...new Set([
    zh?.label,
    ...(zh?.tags || []),
    en?.label,
    ...(en?.tags || []),
  ].filter(Boolean))].join(' ').toLocaleLowerCase('zh-CN');
}

const [english, chinese] = await Promise.all([load('en'), load('zh')]);
const zhByHexcode = new Map(chinese.map((item) => [item.hexcode, item]));
const rows = [];

for (const item of english) {
  if (!Number.isInteger(item.group) || item.group === 2 || !item.unicode) continue;
  const zh = zhByHexcode.get(item.hexcode);
  rows.push([item.unicode, item.group, zh?.label || item.label, searchText(item, zh), item.order || 0]);

  const zhSkins = new Map((zh?.skins || []).map((skin) => [skin.hexcode, skin]));
  for (const skin of item.skins || []) {
    const zhSkin = zhSkins.get(skin.hexcode);
    rows.push([skin.unicode, item.group, zhSkin?.label || skin.label, searchText(skin, zhSkin), skin.order || item.order || 0]);
  }
}

rows.sort((a, b) => a[4] - b[4]);
const output = {
  version: VERSION,
  source: 'emojibase-data',
  emojis: rows.map((row) => row.slice(0, 4)),
};

mkdirSync(dirname(outPath), { recursive: true });
writeFileSync(outPath, `${JSON.stringify(output)}\n`);
console.log(`Wrote ${output.emojis.length} emoji records to ${outPath}`);
