// Stellar 1.44 Wiki Hero Galaxy 参数与 WebGL 降级行为。
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import vm from "node:vm";

const SOURCE_PATH = join(dirname(fileURLToPath(import.meta.url)), "../web/assets/js/plugins/galaxy.js");
const SOURCE = readFileSync(SOURCE_PATH, "utf8");

function recorder() {
  const calls = [];
  const gl = {
    VERTEX_SHADER: 1, FRAGMENT_SHADER: 2, COMPILE_STATUS: 3, LINK_STATUS: 4,
    ARRAY_BUFFER: 5, STATIC_DRAW: 6, FLOAT: 7, BLEND: 8, SRC_ALPHA: 9,
    ONE_MINUS_SRC_ALPHA: 10, COLOR_BUFFER_BIT: 11, TRIANGLES: 12,
    createShader() { return {}; }, shaderSource() {}, compileShader() {},
    getShaderParameter() { return true; }, getShaderInfoLog() { return ""; }, deleteShader() {},
    createProgram() { return {}; }, attachShader() {}, linkProgram() {},
    getProgramParameter() { return true; }, getProgramInfoLog() { return ""; }, deleteProgram() {},
    getUniformLocation(_program, name) { return name; }, getAttribLocation() { return 0; },
    createBuffer() { return {}; }, bindBuffer() {}, bufferData() {}, enableVertexAttribArray() {},
    vertexAttribPointer() {}, useProgram() {}, enable() {}, blendFunc() {}, clearColor() {},
    uniform2f(name, x, y) { calls.push(["uniform2f", name, x, y]); },
    uniform1f(name, value) { calls.push(["uniform1f", name, value]); },
    uniform1i(name, value) { calls.push(["uniform1i", name, value]); },
    uniform3f(name, x, y, z) { calls.push(["uniform3f", name, x, y, z]); },
    viewport() {}, clear() {}, drawArrays() {}, deleteBuffer() {}, getExtension() { return null; },
  };
  return { gl, calls };
}

function createCanvas(rawParams, context = null) {
  const recorded = recorder();
  const background = {
    addEventListener() {}, removeEventListener() {},
    getBoundingClientRect() { return { width: 1200, height: 640, left: 0, top: 0 }; },
  };
  const hero = { addEventListener() {}, removeEventListener() {} };
  const canvas = {
    dataset: {}, parentElement: background, width: 0, height: 0,
    closest() { return hero; },
    getAttribute(name) { return name === "data-galaxy-params" ? rawParams : null; },
    getContext() { return context === "none" ? null : recorded.gl; },
  };
  return { canvas, calls: recorded.calls };
}

function runtime() {
  const frames = [];
  const window = {
    stellar: {}, addEventListener() {}, removeEventListener() {},
    requestAnimationFrame(callback) { frames.push(callback); return frames.length; },
    cancelAnimationFrame() {},
  };
  const document = { hidden: false, addEventListener() {}, removeEventListener() {} };
  vm.runInContext(SOURCE, vm.createContext({ window, document, console }), { filename: SOURCE_PATH });
  return { galaxy: window.stellar.galaxy, frames };
}

function last(calls, method, uniform) {
  return calls.filter((item) => item[0] === method && item[1] === uniform).at(-1);
}

test("Galaxy 缺失参数时使用完整默认值", () => {
  const rt = runtime();
  const target = createCanvas("{}");
  rt.galaxy.mountAll([target.canvas]);
  rt.frames.shift()(1000);

  assert.deepEqual(last(target.calls, "uniform2f", "uFocal"), ["uniform2f", "uFocal", 0.5, 0.5]);
  assert.deepEqual(last(target.calls, "uniform1f", "uDensity"), ["uniform1f", "uDensity", 2]);
  assert.deepEqual(last(target.calls, "uniform1f", "uHueShift"), ["uniform1f", "uHueShift", 140]);
  assert.deepEqual(last(target.calls, "uniform1i", "uMouseRepulsion"), ["uniform1i", "uMouseRepulsion", 1]);
  assert.deepEqual(last(target.calls, "uniform1i", "uTransparent"), ["uniform1i", "uTransparent", 1]);
  assert.equal(target.canvas.dataset.galaxyMounted, "true");
});

test("Galaxy 逐项规范化自定义参数并忽略未知值", () => {
  const rt = runtime();
  const target = createCanvas(JSON.stringify({
    focal: [2, -1], rotation: [0.25, -0.5], starSpeed: 4, density: -1,
    hueShift: -20, speed: 1.5, glowIntensity: "bright", saturation: 0.8,
    mouseRepulsion: false, twinkleIntensity: -0.2, rotationSpeed: -0.3,
    repulsionStrength: 0.4, autoCenterRepulsion: 0.2, transparent: false, unknown: 999,
  }));
  rt.galaxy.mountAll([target.canvas]);
  rt.frames.shift()(1000);

  assert.deepEqual(last(target.calls, "uniform2f", "uFocal"), ["uniform2f", "uFocal", 1, 0]);
  assert.deepEqual(last(target.calls, "uniform1f", "uDensity"), ["uniform1f", "uDensity", 2]);
  assert.deepEqual(last(target.calls, "uniform1f", "uHueShift"), ["uniform1f", "uHueShift", 340]);
  assert.deepEqual(last(target.calls, "uniform1f", "uSpeed"), ["uniform1f", "uSpeed", 1.5]);
  assert.deepEqual(last(target.calls, "uniform1i", "uMouseRepulsion"), ["uniform1i", "uMouseRepulsion", 0]);
  assert.deepEqual(last(target.calls, "uniform1i", "uTransparent"), ["uniform1i", "uTransparent", 0]);
});

test("每个 Canvas 独立读取参数，非法 JSON 和 WebGL 失败均降级", () => {
  const rt = runtime();
  const first = createCanvas("{invalid json");
  const second = createCanvas(JSON.stringify({ density: 5, hueShift: 400 }));
  const unavailable = createCanvas("{}", "none");
  rt.galaxy.mountAll([first.canvas, second.canvas, unavailable.canvas]);

  assert.deepEqual(last(first.calls, "uniform1f", "uDensity"), ["uniform1f", "uDensity", 2]);
  assert.deepEqual(last(second.calls, "uniform1f", "uDensity"), ["uniform1f", "uDensity", 5]);
  assert.deepEqual(last(second.calls, "uniform1f", "uHueShift"), ["uniform1f", "uHueShift", 40]);
  assert.equal(unavailable.canvas.dataset.galaxyMounted, undefined);
});
