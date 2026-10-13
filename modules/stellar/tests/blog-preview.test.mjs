import { readFileSync } from 'node:fs';
import vm from 'node:vm';
import assert from 'node:assert/strict';
import test from 'node:test';

const source = readFileSync(new URL('../web/assets/js/blog-preview.js', import.meta.url), 'utf8');
function fixture() {
  const listeners = new Map();
  const messages = [];
  const frames = new Map();
  let nextFrame = 1;
  let disconnected = false;
  const parent = {postMessage: value => messages.push(value)};
  const window = {parent, addEventListener: (name, callback) => listeners.set(name, callback), removeEventListener: name => listeners.delete(name)};
  const scrollingElement = {scrollHeight: 1400, clientHeight: 400, scrollTop: 250};
  const document = {documentElement: {dataset: {stellarPreviewToken: 'current-preview-token'}}, body: {getBoundingClientRect: () => ({height: 600})}, querySelector: () => null, scrollingElement, addEventListener() {}, removeEventListener() {}};
  vm.runInNewContext(source, {window, document, ResizeObserver: class {observe() {} disconnect() {disconnected = true;}}, requestAnimationFrame: callback => {const id = nextFrame++; frames.set(id, callback); return id;}, cancelAnimationFrame: id => frames.delete(id)});
  const flush = () => {const pending = [...frames.values()]; frames.clear(); pending.forEach(callback => callback());};
  return {listeners, messages, parent, scrollingElement, flush, disconnected: () => disconnected};
}

test('opaque preview bridge accepts only parent source and current generation token', () => {
  const f = fixture();
  const onMessage = f.listeners.get('message');
  for (const event of [
    {source: {}, data: {type: 'stellar-preview-scroll-to', token: 'current-preview-token', ratio: 0.8}},
    {source: f.parent, data: {type: 'stellar-preview-scroll-to', token: 'old-preview-token', ratio: 0.8}},
    {source: f.parent, data: {type: 'stellar-preview-scroll-to', token: 'current-preview-token', ratio: '0.8'}},
  ]) onMessage(event);
  assert.equal(f.scrollingElement.scrollTop, 250);
  onMessage({source: f.parent, data: {type: 'stellar-preview-scroll-to', token: 'current-preview-token', ratio: 0.8}});
  assert.equal(f.scrollingElement.scrollTop, 800);
});

test('preview bridge reports bounded resize and scroll with token and removes listeners', () => {
  const f = fixture(); f.flush();
  assert.equal(f.messages[0].type, 'component-preview-resize');
  assert.equal(f.messages[0].height, 600);
  assert.equal(f.messages[0].token, 'current-preview-token');
  f.listeners.get('scroll')(); f.listeners.get('scroll')(); f.flush();
  assert.equal(f.messages.filter(value => value.type === 'stellar-preview-scroll').length, 1);
  assert.equal(f.messages.at(-1).ratio, 0.25);
  f.listeners.get('pagehide')();
  assert.equal(f.disconnected(), true);
  assert.equal(f.listeners.has('message'), false);
  assert.equal(f.listeners.has('scroll'), false);
});
