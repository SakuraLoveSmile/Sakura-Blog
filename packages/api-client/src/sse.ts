// 最小 SSE 解析器：按 WHATWG 规范逐行解析 fetch 流，只关心 data 与 event 字段。
// EventSource 只支持 GET，而写作助手要用 POST 请求体，所以直接解析响应流。

export interface SSEMessage {
  /** event 字段；没有时是规范规定的 "message"。 */
  event: string;
  /** 同一事件里多个 data 行用 "\n" 连接。 */
  data: string;
}

export interface SSEParser {
  /** 喂入一段已解码的文本，可以在任意位置切开（包括 \r\n 之间）。 */
  push(chunk: string): void;
  /** 流结束时调用：结尾孤立的 \r 此时才能确定是行结束符。未以空行收尾的事件按规范丢弃。 */
  end(): void;
}

export function createSSEParser(onMessage: (message: SSEMessage) => void): SSEParser {
  let buffer = "";
  let started = false;
  let data: string[] = [];
  let event = "";

  function dispatch() {
    if (data.length > 0) onMessage({ event: event || "message", data: data.join("\n") });
    data = [];
    event = "";
  }

  function line(text: string) {
    if (text === "") {
      dispatch();
      return;
    }
    if (text.startsWith(":")) return; // 注释行，服务端用来保活
    const colon = text.indexOf(":");
    const field = colon < 0 ? text : text.slice(0, colon);
    let value = colon < 0 ? "" : text.slice(colon + 1);
    if (value.startsWith(" ")) value = value.slice(1);
    if (field === "data") data.push(value);
    else if (field === "event") event = value;
    // id、retry 对一次性的 POST 流没有意义，忽略。
  }

  function push(chunk: string) {
    if (!started && chunk !== "") {
      started = true;
      if (chunk.charCodeAt(0) === 0xfeff) chunk = chunk.slice(1); // 流开头的 BOM
    }
    buffer += chunk;
    let start = 0;
    for (let i = 0; i < buffer.length; i++) {
      const ch = buffer[i];
      if (ch === "\n") {
        line(buffer.slice(start, i));
        start = i + 1;
      } else if (ch === "\r") {
        // 结尾的 \r 可能是 \r\n 的前半，留到下一段再判断。
        if (i === buffer.length - 1) break;
        line(buffer.slice(start, i));
        if (buffer[i + 1] === "\n") i++;
        start = i + 1;
      }
    }
    buffer = buffer.slice(start);
  }

  return {
    push,
    end() {
      if (buffer.endsWith("\r")) push("\n");
    },
  };
}
