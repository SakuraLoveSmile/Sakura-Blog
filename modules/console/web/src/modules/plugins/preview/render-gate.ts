// 预览渲染闸门（Spec 63 + BLG-20261031-03）：防抖 + 请求序列号 + 有界缓存 + 可选并发上限。
// 快速连续编辑示例时只执行最后一次渲染；异步返回后按序列号丢弃旧结果；
// 相同 profile + markdown + context 指纹命中缓存直接应用，不再请求。
// 纯 TS，可脱离 Vue 单测（tests/plugin-preview*.test.ts）。

export type PreviewStatus = "idle" | "loading" | "ready" | "updating" | "error";

export interface PreviewResult {
  /** 渲染后的片段 HTML（goldmark 产物） */
  html: string;
  /** 可放进 iframe srcdoc 的完整文档 */
  document: string;
  /** 当前文档的 token（高度/滚动消息校验用） */
  token: string;
}

export interface PreviewState {
  status: PreviewStatus;
  result: PreviewResult | null;
  error: string;
}

export interface PreviewRequest {
  markdown: string;
  /** null 表示真实站点上下文（正文 NodeView）；缺省由调用方决定 profile */
  profile?: string | null;
  /** 参与缓存指纹的渲染上下文（标题/slug/配置等） */
  metadata?: Record<string, unknown>;
  /** 覆盖默认缓存键（一般不需要） */
  cacheKey?: string;
  delayMs?: number;
  immediate?: boolean;
  /** 重试时跳过缓存，强制重新请求 */
  bypassCache?: boolean;
}

export type PreviewRunner = (
  markdown: string,
  profile: string | undefined,
  metadata?: Record<string, unknown>,
) => Promise<PreviewResult>;

// ---------- 稳定的上下文指纹 ----------

function stableValue(value: unknown, seen: WeakSet<object>): unknown {
  if (value === null || typeof value !== "object") return value;
  if (seen.has(value)) return "[circular]";
  seen.add(value);
  if (Array.isArray(value)) return value.map((item) => stableValue(item, seen));
  const source = value as Record<string, unknown>;
  const out: Record<string, unknown> = {};
  for (const key of Object.keys(source).sort()) out[key] = stableValue(source[key], seen);
  return out;
}

/** 上下文指纹：键排序后的稳定 JSON 的 FNV-1a 短哈希（标题/slug/配置变化即失效）。 */
export function previewFingerprint(value: unknown): string {
  let text: string;
  try {
    text = JSON.stringify(stableValue(value, new WeakSet<object>())) ?? String(value);
  } catch {
    text = String(value);
  }
  let hash = 0x811c9dc5;
  for (let i = 0; i < text.length; i++) {
    hash ^= text.charCodeAt(i);
    hash = Math.imul(hash, 0x01000193) >>> 0;
  }
  return `${hash.toString(16)}-${text.length}`;
}

/** 缓存键：profile + markdown + 上下文指纹。 */
export function previewCacheKey(
  profile: string | null | undefined,
  markdown: string,
  metadata?: unknown,
): string {
  return `${profile || "real"}|${previewFingerprint(markdown)}|${previewFingerprint(metadata ?? null)}`;
}

// ---------- 有界 LRU 缓存 ----------

export class PreviewCache {
  private entries = new Map<string, PreviewResult>();
  private readonly capacity: number;

  constructor(capacity = 20) {
    this.capacity = capacity;
  }

  get(key: string): PreviewResult | undefined {
    const value = this.entries.get(key);
    if (!value) return undefined;
    // LRU：命中后移到末尾
    this.entries.delete(key);
    this.entries.set(key, value);
    return value;
  }

  set(key: string, value: PreviewResult): void {
    if (this.entries.has(key)) this.entries.delete(key);
    this.entries.set(key, value);
    const limit = Math.max(1, this.capacity);
    while (this.entries.size > limit) {
      const oldest = this.entries.keys().next().value;
      if (oldest === undefined) break;
      this.entries.delete(oldest);
    }
  }

  delete(key: string): boolean {
    return this.entries.delete(key);
  }

  clear(): void {
    this.entries.clear();
  }

  get size(): number {
    return this.entries.size;
  }
}

/** 共享缓存：正文 NodeView、侧栏面板、Studio 与顶栏气泡复用同一批预览结果。 */
export const sharedPreviewCache = new PreviewCache(20);

// ---------- 模块级并发上限（避免首屏 N 个整站渲染同时打满） ----------

export class PreviewQueue {
  private active = 0;
  private waiters: Array<() => void> = [];
  private readonly limit: number;

  constructor(limit = 2) {
    this.limit = limit;
  }

  /**
   * 获取一个执行位。shouldRun 在真正 dequeued 时调用，过期的排队请求会被跳过，
   * 因此可见标签不会被已取消的旧请求长期堵住。
   */
  acquire(shouldRun: () => boolean = () => true): Promise<boolean> {
    if (this.active < this.limit) {
      this.active++;
      return Promise.resolve(true);
    }
    return new Promise<boolean>((resolve) => {
      this.waiters.push(() => {
        if (!shouldRun()) {
          resolve(false);
          this.drain();
          return;
        }
        this.active++;
        resolve(true);
      });
    });
  }

  release(): void {
    this.active = Math.max(0, this.active - 1);
    this.drain();
  }

  private drain(): void {
    while (this.active < this.limit && this.waiters.length > 0) {
      const next = this.waiters.shift();
      next?.();
    }
  }
}

/** 所有标签预览共用的并发闸（上限 2）。 */
export const sharedPreviewQueue = new PreviewQueue(2);

// ---------- 调度器 ----------

export interface PreviewSchedulerOptions {
  delayMs?: number;
  cache?: PreviewCache;
  queue?: PreviewQueue;
  run: PreviewRunner;
  onChange: (state: PreviewState) => void;
}

/**
 * 共享预览调度器：
 * - schedule 合并防抖，不清空当前结果；只有 clear 才清空。
 * - 有结果时状态是 updating（保留旧文档），无结果时是 loading。
 * - 命中缓存同步应用；迟到响应按序列号丢弃；失败保留旧结果并暴露 error。
 */
export class PreviewScheduler {
  private seq = 0;
  private timer: ReturnType<typeof setTimeout> | undefined;
  private disposed = false;
  private result: PreviewResult | null = null;
  private status: PreviewStatus = "idle";
  private error = "";
  private inflightKey: string | null = null;
  private lastRequest: PreviewRequest | null = null;
  private readonly delayMs: number;
  private readonly cache: PreviewCache;
  private readonly opts: PreviewSchedulerOptions;

  constructor(opts: PreviewSchedulerOptions) {
    this.opts = opts;
    this.delayMs = opts.delayMs ?? 400;
    this.cache = opts.cache ?? sharedPreviewCache;
  }

  get state(): PreviewState {
    return { status: this.status, result: this.result, error: this.error };
  }

  get latestSeq(): number {
    return this.seq;
  }

  schedule(request: PreviewRequest): void {
    if (this.disposed) return;
    const markdown = request.markdown ?? "";
    if (!markdown.trim()) {
      this.clear();
      return;
    }
    const key = request.cacheKey ?? previewCacheKey(request.profile, markdown, request.metadata);
    this.lastRequest = { ...request };
    // 同一输入仍在途：保持旧文档与更新中状态，等这次响应回来，避免重复请求。
    if (this.inflightKey === key && (this.status === "loading" || this.status === "updating")) {
      this.cancelTimer();
      return;
    }
    this.seq++;
    this.cancelTimer();
    this.inflightKey = null;
    if (!request.bypassCache) {
      const cached = this.cache.get(key);
      if (cached) {
        this.result = cached;
        this.status = "ready";
        this.error = "";
        this.emit();
        return;
      }
    }
    this.status = this.result ? "updating" : "loading";
    this.error = "";
    this.emit();
    const seq = this.seq;
    const delay = request.immediate ? 0 : (request.delayMs ?? this.delayMs);
    this.timer = setTimeout(() => {
      this.timer = undefined;
      void this.run(seq, key, request);
    }, delay);
  }

  clear(): void {
    this.seq++;
    this.cancelTimer();
    this.inflightKey = null;
    this.lastRequest = null;
    this.result = null;
    this.status = "idle";
    this.error = "";
    this.emit();
  }

  /** 失败后重试同一请求（跳过缓存）。 */
  retry(): void {
    if (!this.lastRequest || this.disposed) return;
    this.schedule({ ...this.lastRequest, immediate: true, bypassCache: true });
  }

  /** 主动失效缓存（不传 key 时清空共享缓存）。 */
  invalidate(cacheKey?: string): void {
    if (cacheKey) this.cache.delete(cacheKey);
    else this.cache.clear();
  }

  dispose(): void {
    this.disposed = true;
    this.seq++;
    this.cancelTimer();
    this.inflightKey = null;
  }

  private cancelTimer(): void {
    if (this.timer !== undefined) {
      clearTimeout(this.timer);
      this.timer = undefined;
    }
  }

  private emit(): void {
    this.opts.onChange({ status: this.status, result: this.result, error: this.error });
  }

  private async run(seq: number, key: string, request: PreviewRequest): Promise<void> {
    if (this.disposed || seq !== this.seq) return;
    this.inflightKey = key;
    const current = () => !this.disposed && seq === this.seq;
    const queue = this.opts.queue;
    let acquired = false;
    if (queue) {
      acquired = await queue.acquire(current);
      if (!acquired) return;
      if (!current()) {
        queue.release();
        return;
      }
    }
    try {
      const result = await this.opts.run(request.markdown, request.profile ?? undefined, request.metadata);
      if (!current()) return;
      this.cache.set(key, result);
      this.result = result;
      this.status = "ready";
      this.error = "";
      this.emit();
    } catch {
      if (!current()) return;
      this.status = "error";
      this.error = "预览渲染失败";
      this.emit();
    } finally {
      if (this.inflightKey === key) this.inflightKey = null;
      if (acquired) queue?.release();
    }
  }
}

// ---------- 兼容旧调用方的轻量防抖门闸（MarkdownEditor 等未迁移路径） ----------

/**
 * 旧版 RenderGate 保留：只做防抖 + 序列号，不持有结果。
 * 新代码请使用 PreviewScheduler（保留旧文档/缓存/错误态）。
 */
export class RenderGate {
  private seq = 0;
  private timer: ReturnType<typeof setTimeout> | undefined;
  private delay: number;

  constructor(delay = 500) {
    this.delay = delay;
  }

  /** 调度一次渲染；immediate=true 跳过防抖立即执行。返回本次序列号。 */
  schedule(run: () => Promise<void> | void, immediate = false): number {
    const seq = ++this.seq;
    if (this.timer !== undefined) clearTimeout(this.timer);
    this.timer = setTimeout(
      () => {
        this.timer = undefined;
        void this.run(run);
      },
      immediate ? 0 : this.delay,
    );
    return seq;
  }

  private async run(run: () => Promise<void> | void): Promise<void> {
    try {
      await run();
    } catch {
      // 渲染错误由调用方在自己的 try/catch 内处理，这里只保证门闸不吞异常后悬挂
    }
  }

  /** 该序列号是否仍是最新（旧请求结果应被丢弃） */
  isCurrent(seq: number): boolean {
    return seq === this.seq;
  }

  /** 最新序列号（测试用） */
  get latestSeq(): number {
    return this.seq;
  }

  invalidate(): void {
    this.seq++;
    if (this.timer !== undefined) {
      clearTimeout(this.timer);
      this.timer = undefined;
    }
  }

  dispose(): void {
    this.invalidate();
  }
}
