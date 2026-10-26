// 预览渲染闸门（Spec 63）：防抖 + 请求序列号。
// 快速连续编辑示例时，只执行最后一次渲染；异步返回后按序列号丢弃旧结果，
// 保证旧请求的响应不会覆盖新预览。纯 TS，可脱离 Vue 单测（tests/plugin-preview.test.ts）。

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
