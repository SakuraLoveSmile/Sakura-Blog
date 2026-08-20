// Mermaid loader behavior: the custom Stellar stylesheet is opt-in and must
// finish loading before Mermaid renders; the official theme path loads no
// Stellar CSS.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import vm from 'node:vm';

const SRC_PATH = join(dirname(fileURLToPath(import.meta.url)), '../web/assets/js/math-mermaid-loader.js');

async function runLoader(styleOptimization) {
  const events = [];
  const elements = [];
  const mermaid = {
    initialize(config) { events.push(`initialize:${config.theme}`); },
    run() { events.push('run'); return Promise.resolve(); },
  };
  const head = {
    appendChild(el) {
      elements.push(el);
      if (el.tagName === 'SCRIPT') events.push(`script:${el.src}`);
      if (el.tagName === 'LINK') events.push(`link:${el.href}`);
      setTimeout(() => el.onload?.(), 0);
      return el;
    },
  };
  const document = {
    readyState: 'complete',
    head,
    createElement(tag) {
      return {
        tagName: String(tag).toUpperCase(),
        setAttribute() {},
        remove() {},
      };
    },
    querySelectorAll(selector) { return selector === '.md-text' ? [] : []; },
    querySelector(selector) { return selector === '.md-text .mermaid, .mermaid' ? {} : null; },
    addEventListener() {},
  };
  const sandbox = {
    console: { log() {}, warn() {}, error() {} },
    document,
    mermaid,
    setTimeout,
    clearTimeout,
    Promise,
    __stellarPlugins: {
      mermaid: { enable: true, style_optimization: styleOptimization, theme: 'neutral' },
    },
  };
  sandbox.window = sandbox;
  vm.createContext(sandbox);
  vm.runInContext(readFileSync(SRC_PATH, 'utf8'), sandbox, { filename: SRC_PATH });
  await new Promise((resolve) => setTimeout(resolve, 15));
  return events;
}

test('Mermaid style_optimization=false uses the official theme without Stellar CSS', async () => {
  const events = await runLoader(false);
  assert.ok(events.includes('script:/assets/vendor/mermaid/mermaid.min.js'));
  assert.ok(events.includes('initialize:neutral'));
  assert.ok(events.includes('run'));
  assert.equal(events.some((event) => event === 'link:/assets/css/mermaid.css'), false);
});

test('Mermaid style_optimization=true loads mermaid.css before rendering', async () => {
  const events = await runLoader(true);
  const css = events.indexOf('link:/assets/css/mermaid.css');
  const initialize = events.findIndex((event) => event.startsWith('initialize:'));
  const run = events.indexOf('run');
  assert.ok(css >= 0, `events: ${events.join(', ')}`);
  assert.ok(css < initialize && initialize < run, `events: ${events.join(', ')}`);
});
