<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from "vue";
import { Icon } from "@iconify/vue";
import {
  getApiErrorMessage,
  ragApi,
  type RAGCitation,
  type RAGStatusResponse,
} from "@/api";
import { Toast, VButton, VPageHeader, VStatusDot } from "@/components/ui";

interface ChatMessage {
  id: string;
  role: "user" | "assistant";
  content: string;
  citations?: RAGCitation[];
  timestamp: string;
  error?: boolean;
}

const status = ref<RAGStatusResponse | null>(null);
const question = ref("");
const messages = ref<ChatMessage[]>([]);
const loading = ref(false);
const rebuilding = ref(false);
const statusLoading = ref(true);
const copiedId = ref<string | null>(null);
const chatContainerRef = ref<HTMLElement | null>(null);
let pollTimer: ReturnType<typeof setInterval> | null = null;

const suggestedPrompts = [
  {
    icon: "ri:database-2-line",
    tag: "性能调优",
    title: "SQLite 性能优化建议",
    query: "本站关于 SQLite 性能优化有哪些关键建议与实践？",
  },
  {
    icon: "ri:rocket-2-line",
    tag: "架构部署",
    title: "系统编译与自动化部署",
    query: "博客系统是如何进行编译构建与自动化部署的？",
  },
  {
    icon: "ri:palette-line",
    tag: "主题特色",
    title: "Stellar 主题标签插件",
    query: "Stellar 主题有哪些特色的标签插件（Shortcodes）与语法？",
  },
  {
    icon: "ri:article-line",
    tag: "内容概览",
    title: "总结最近公开文章",
    query: "总结一下本站最近公开的文章主要涵盖了哪些主题和内容？",
  },
];

const progress = computed(() => {
  if (!status.value || status.value.total <= 0) return 0;
  return Math.min(100, Math.round((status.value.done / status.value.total) * 100));
});

const statusLabel = computed(() => {
  switch (status.value?.status) {
    case "building":
      return "索引构建中";
    case "error":
      return "构建异常";
    default:
      return "知识库就绪";
  }
});

const statusDot = computed<"default" | "success" | "warning" | "error">(() => {
  switch (status.value?.status) {
    case "building":
      return "warning";
    case "error":
      return "error";
    default:
      return "success";
  }
});

function formatTime(value?: string) {
  if (!value) return "暂无记录";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString("zh-CN", { hour12: false });
}

function escapeHtml(str: string): string {
  return str
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#039;");
}

function renderInline(text: string): string {
  // Inline code: `code`
  let out = text.replace(/`([^`]+)`/g, (_m, code) => {
    return `<code class="ai-inline-code">${escapeHtml(code)}</code>`;
  });

  // Bold: **text** or __text__
  out = out.replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>");
  out = out.replace(/__([^_]+)__/g, "<strong>$1</strong>");

  // Italic: *text* or _text_
  out = out.replace(/(^|[^\*])\*([^*]+)\*(?!\*)/g, "$1<em>$2</em>");
  out = out.replace(/(^|[^_])_([^_]+)_(?!_)/g, "$1<em>$2</em>");

  // Citations: [1], [2], [S1], [S2] etc.
  out = out.replace(/\[([S0-9]+)\](?!\()/g, '<span class="ai-citation-pill">[$1]</span>');

  // Links: [text](url)
  out = out.replace(
    /\[([^\]]+)\]\((https?:\/\/[^\s\)]+)\)/g,
    '<a href="$2" target="_blank" rel="noopener noreferrer" class="ai-link">$1</a>',
  );

  return out;
}

function renderMarkdown(content: string): string {
  if (!content) return "";

  const blocks: string[] = [];
  const lines = content.split("\n");
  let inCodeBlock = false;
  let codeBlockLang = "";
  let codeBlockLines: string[] = [];
  let currentParagraphLines: string[] = [];
  let inListType: "ul" | "ol" | null = null;
  let listItems: string[] = [];

  function flushParagraph() {
    if (currentParagraphLines.length > 0) {
      const pText = currentParagraphLines.join(" ").trim();
      if (pText) {
        blocks.push(`<p>${renderInline(escapeHtml(pText))}</p>`);
      }
      currentParagraphLines = [];
    }
  }

  function flushList() {
    if (inListType && listItems.length > 0) {
      const tag = inListType === "ol" ? "ol" : "ul";
      const itemsHtml = listItems
        .map((item) => `<li>${renderInline(item)}</li>`)
        .join("");
      blocks.push(`<${tag} class="ai-${tag}">${itemsHtml}</${tag}>`);
      listItems = [];
      inListType = null;
    }
  }

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];

    // Code block fence ```
    const fenceMatch = line.match(/^```(\w*)/);
    if (fenceMatch) {
      if (inCodeBlock) {
        const codeContent = escapeHtml(codeBlockLines.join("\n"));
        blocks.push(
          `<pre class="ai-code-block"><code class="language-${codeBlockLang}">${codeContent}</code></pre>`,
        );
        inCodeBlock = false;
        codeBlockLines = [];
      } else {
        flushParagraph();
        flushList();
        inCodeBlock = true;
        codeBlockLang = fenceMatch[1] || "";
        codeBlockLines = [];
      }
      continue;
    }

    if (inCodeBlock) {
      codeBlockLines.push(line);
      continue;
    }

    // Blank line
    if (!line.trim()) {
      flushParagraph();
      flushList();
      continue;
    }

    // Headings: #, ##, ###, ####
    const headingMatch = line.match(/^(#{1,6})\s+(.*)$/);
    if (headingMatch) {
      flushParagraph();
      flushList();
      const level = headingMatch[1].length;
      const hText = headingMatch[2].trim();
      const safeHeading = renderInline(escapeHtml(hText));
      blocks.push(`<h${level} class="ai-heading ai-h${level}">${safeHeading}</h${level}>`);
      continue;
    }

    // Blockquote: > text
    const quoteMatch = line.match(/^>\s?(.*)$/);
    if (quoteMatch) {
      flushParagraph();
      flushList();
      const qText = renderInline(escapeHtml(quoteMatch[1]));
      blocks.push(`<blockquote class="ai-quote">${qText}</blockquote>`);
      continue;
    }

    // Ordered list: 1. text, 2. text
    const olMatch = line.match(/^(\d+)\.\s+(.*)$/);
    if (olMatch) {
      flushParagraph();
      if (inListType !== "ol") {
        flushList();
        inListType = "ol";
      }
      listItems.push(escapeHtml(olMatch[2]));
      continue;
    }

    // Unordered list: - text, * text, • text
    const ulMatch = line.match(/^[-*•]\s+(.*)$/);
    if (ulMatch) {
      flushParagraph();
      if (inListType !== "ul") {
        flushList();
        inListType = "ul";
      }
      listItems.push(escapeHtml(ulMatch[1]));
      continue;
    }

    // Regular line -> accumulate into paragraph
    currentParagraphLines.push(line);
  }

  if (inCodeBlock && codeBlockLines.length > 0) {
    const codeContent = escapeHtml(codeBlockLines.join("\n"));
    blocks.push(`<pre class="ai-code-block"><code>${codeContent}</code></pre>`);
  }

  flushParagraph();
  flushList();

  return blocks.join("");
}

function scrollToBottom() {
  nextTick(() => {
    if (chatContainerRef.value) {
      chatContainerRef.value.scrollTop = chatContainerRef.value.scrollHeight;
    }
  });
}

async function loadStatus() {
  try {
    const { data } = await ragApi.status();
    status.value = data;
  } catch {
    // transport handles normalized toast
  } finally {
    statusLoading.value = false;
  }
}

async function rebuild() {
  rebuilding.value = true;
  try {
    const { data } = await ragApi.rebuild();
    status.value = data;
    Toast.success("已触发知识库全量索引重建");
    await loadStatus();
  } catch (error) {
    Toast.error(getApiErrorMessage(error, "触发重建失败"));
  } finally {
    rebuilding.value = false;
  }
}

async function handleSend(text?: string) {
  const queryText = (text ?? question.value).trim();
  if (!queryText) {
    Toast.warning("请输入您的问题");
    return;
  }
  if (loading.value) return;

  const userMsgId = `user-${Date.now()}`;
  const now = new Date().toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" });

  messages.value.push({
    id: userMsgId,
    role: "user",
    content: queryText,
    timestamp: now,
  });

  question.value = "";
  loading.value = true;
  scrollToBottom();

  try {
    const { data } = await ragApi.ask(queryText);
    messages.value.push({
      id: `ai-${Date.now()}`,
      role: "assistant",
      content: data.answer,
      citations: data.citations || [],
      timestamp: new Date().toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" }),
    });
  } catch (error) {
    const errMsg = getApiErrorMessage(error, "AI 检索问答失败，请稍后重试");
    messages.value.push({
      id: `ai-err-${Date.now()}`,
      role: "assistant",
      content: `⚠️ ${errMsg}`,
      timestamp: new Date().toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" }),
      error: true,
    });
    Toast.error(errMsg);
  } finally {
    loading.value = false;
    scrollToBottom();
  }
}

function selectPrompt(prompt: string) {
  void handleSend(prompt);
}

function clearChat() {
  messages.value = [];
  Toast.info("对话记录已清空");
}

async function copyAnswer(id: string, text: string) {
  try {
    await navigator.clipboard.writeText(text);
    copiedId.value = id;
    Toast.success("已复制回答内容");
    setTimeout(() => {
      if (copiedId.value === id) copiedId.value = null;
    }, 2000);
  } catch {
    Toast.error("复制失败，请手动选择复制");
  }
}

onMounted(() => {
  loadStatus();
  pollTimer = setInterval(() => {
    if (status.value?.status === "building") void loadStatus();
  }, 2500);
});

onBeforeUnmount(() => {
  if (pollTimer) clearInterval(pollTimer);
});
</script>

<template>
  <div class="assistant-page">
    <VPageHeader title="AI 助手">
      <template #icon>
        <div class="header-icon-box">
          <Icon icon="ri:sparkling-2-fill" class="w-5 h-5 text-primary animate-pulse" />
        </div>
      </template>
      <template #actions>
        <div class="flex items-center gap-2">
          <VButton
            v-if="messages.length > 0"
            type="secondary"
            size="sm"
            @click="clearChat"
          >
            <template #icon><Icon icon="ri:delete-bin-line" class="w-3.5 h-3.5" /></template>
            清空对话
          </VButton>
          <VButton
            type="primary"
            size="sm"
            :loading="rebuilding || status?.status === 'building'"
            :disabled="status?.status === 'building'"
            @click="rebuild"
          >
            <template #icon><Icon icon="ri:refresh-line" class="w-3.5 h-3.5" /></template>
            {{ status?.status === 'building' ? '知识库构建中...' : '重建知识库' }}
          </VButton>
        </div>
      </template>
    </VPageHeader>

    <div class="assistant-layout">
      <!-- 左侧：AI 对话工作区 -->
      <div class="chat-workspace">
        <!-- 聊天区域容器 -->
        <div class="chat-card apple-card">
          <!-- 消息列表滚动区 -->
          <div ref="chatContainerRef" class="chat-messages-scroll">
            <!-- 空状态 / 欢迎引导与快捷提问 -->
            <div v-if="messages.length === 0" class="welcome-hero">
              <div class="welcome-badge">
                <Icon icon="ri:brain-line" class="w-3.5 h-3.5 text-primary" />
                <span>RAG 本地知识库增强检索</span>
              </div>
              <h2 class="welcome-title">有什么关于博客的问题可以帮您？</h2>
              <p class="welcome-subtitle">
                AI 助手已索引全站公开文章与单页，结合本地语义向量检索，为您提供精准解答与溯源引用。
              </p>

              <!-- 推荐问题卡片 -->
              <div class="suggested-grid">
                <button
                  v-for="item in suggestedPrompts"
                  :key="item.title"
                  type="button"
                  class="suggested-card"
                  @click="selectPrompt(item.query)"
                >
                  <div class="suggested-card-header">
                    <span class="suggested-icon">
                      <Icon :icon="item.icon" class="w-4 h-4 text-primary" />
                    </span>
                    <span class="suggested-tag">{{ item.tag }}</span>
                  </div>
                  <div class="suggested-card-title">{{ item.title }}</div>
                  <div class="suggested-card-query">{{ item.query }}</div>
                </button>
              </div>
            </div>

            <!-- 消息列表 -->
            <div v-else class="messages-list">
              <div
                v-for="msg in messages"
                :key="msg.id"
                class="message-row"
                :class="`is-${msg.role}`"
              >
                <!-- 头像 -->
                <div class="message-avatar" :class="`is-${msg.role}`">
                  <Icon
                    :icon="msg.role === 'user' ? 'ri:user-3-line' : 'ri:sparkling-2-fill'"
                    class="w-4 h-4"
                  />
                </div>

                <!-- 气泡内容 -->
                <div class="message-bubble-wrap">
                  <div class="message-meta">
                    <span class="message-author">{{ msg.role === 'user' ? '您' : '博客助手' }}</span>
                    <span class="message-time">{{ msg.timestamp }}</span>
                  </div>

                  <div class="message-bubble" :class="{ 'is-error': msg.error }">
                    <!-- AI 回答支持富文本 Markdown 解析渲染 -->
                    <div
                      v-if="msg.role === 'assistant' && !msg.error"
                      class="ai-rendered-content"
                      v-html="renderMarkdown(msg.content)"
                    />
                    <!-- 用户提问或报错直接原样显示 -->
                    <div v-else class="message-text">{{ msg.content }}</div>

                    <!-- 引用来源列表 -->
                    <div v-if="msg.citations && msg.citations.length > 0" class="citations-section">
                      <div class="citations-header">
                        <Icon icon="ri:book-read-line" class="w-3.5 h-3.5 text-primary" />
                        <span>参考来源 ({{ msg.citations.length }} 篇相关文档)</span>
                      </div>
                      <div class="citations-grid">
                        <a
                          v-for="cit in msg.citations"
                          :key="cit.id"
                          :href="cit.url"
                          target="_blank"
                          rel="noopener noreferrer"
                          class="citation-card"
                        >
                          <span class="citation-badge">[{{ cit.id }}]</span>
                          <span class="citation-title">{{ cit.title }}</span>
                          <Icon icon="ri:arrow-right-up-line" class="citation-link-icon" />
                        </a>
                      </div>
                    </div>
                  </div>

                  <!-- 消息工具条 -->
                  <div v-if="msg.role === 'assistant' && !msg.error" class="message-actions">
                    <button
                      type="button"
                      class="msg-action-btn"
                      :class="{ 'is-active': copiedId === msg.id }"
                      @click="copyAnswer(msg.id, msg.content)"
                    >
                      <Icon :icon="copiedId === msg.id ? 'ri:check-line' : 'ri:file-copy-line'" class="w-3.5 h-3.5" />
                      <span>{{ copiedId === msg.id ? '已复制' : '复制回答' }}</span>
                    </button>
                  </div>
                </div>
              </div>

              <!-- AI 思考等待态 -->
              <div v-if="loading" class="message-row is-assistant is-thinking">
                <div class="message-avatar is-assistant">
                  <Icon icon="ri:sparkling-2-fill" class="w-4 h-4 animate-spin text-primary" />
                </div>
                <div class="message-bubble-wrap">
                  <div class="message-meta">
                    <span class="message-author">博客助手</span>
                    <span class="message-time">思考中...</span>
                  </div>
                  <div class="message-bubble thinking-bubble">
                    <div class="thinking-dots">
                      <span class="dot" />
                      <span class="dot" />
                      <span class="dot" />
                    </div>
                    <span class="thinking-text">正在检索知识库向量并组织回答...</span>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- 底部输入坞 -->
          <div class="chat-input-dock">
            <div class="input-box-container">
              <textarea
                v-model="question"
                class="chat-textarea"
                rows="2"
                maxlength="1000"
                placeholder="询问关于本站的任何内容，如：博客如何开启双因素认证？"
                :disabled="loading"
                @keydown.meta.enter.exact.prevent="() => handleSend()"
                @keydown.ctrl.enter.exact.prevent="() => handleSend()"
              />

              <div class="input-toolbar">
                <div class="input-hints">
                  <span class="kbd-hint"><kbd>⌘</kbd> / <kbd>Ctrl</kbd> + <kbd>Enter</kbd> 发送</span>
                  <span class="char-count">{{ question.length }} / 1000</span>
                </div>
                <VButton
                  type="primary"
                  size="sm"
                  class="send-btn"
                  :loading="loading"
                  :disabled="!question.trim() || loading"
                  @click="() => handleSend()"
                >
                  <template #icon><Icon icon="ri:send-plane-2-fill" class="w-3.5 h-3.5" /></template>
                  <span>发送</span>
                </VButton>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- 右侧：知识库状态与面板 -->
      <aside class="assistant-sidebar">
        <!-- 知识库核心状态卡 -->
        <div class="apple-card sidebar-card">
          <div class="sidebar-card-header">
            <div class="flex items-center gap-2">
              <div class="w-7 h-7 rounded-lg bg-primary/10 flex items-center justify-center text-primary">
                <Icon icon="ri:database-2-line" class="w-4 h-4" />
              </div>
              <div>
                <h3 class="sidebar-title">知识库状态</h3>
                <p class="sidebar-subtitle">RAG Vector Store</p>
              </div>
            </div>

            <div class="status-pill">
              <VStatusDot :type="statusDot" />
              <span class="text-xs font-medium">{{ statusLabel }}</span>
            </div>
          </div>

          <div v-if="statusLoading" class="sidebar-loading">
            <Icon icon="ri:loader-4-line" class="w-5 h-5 animate-spin text-muted-foreground" />
            <span>读取知识库索引中...</span>
          </div>

          <div v-else-if="status" class="sidebar-content">
            <!-- 构建进度条（构建中时可见） -->
            <div v-if="status.status === 'building'" class="building-progress-box">
              <div class="flex items-center justify-between text-xs mb-1.5">
                <span class="text-primary font-medium">向量索引构建中</span>
                <span class="font-mono text-muted-foreground">{{ progress }}% ({{ status.done }}/{{ status.total }})</span>
              </div>
              <div class="progress-track">
                <div class="progress-fill" :style="{ width: `${progress}%` }" />
              </div>
            </div>

            <!-- Bento 统计指标格 -->
            <div class="bento-stats-grid">
              <div class="bento-stat-item">
                <div class="bento-stat-label">
                  <Icon icon="ri:file-text-line" class="w-3.5 h-3.5 text-primary" />
                  <span>公开文档</span>
                </div>
                <div class="bento-stat-val">{{ status.document_count }}</div>
              </div>

              <div class="bento-stat-item">
                <div class="bento-stat-label">
                  <Icon icon="ri:node-tree" class="w-3.5 h-3.5 text-state-warning" />
                  <span>文本分块</span>
                </div>
                <div class="bento-stat-val">{{ status.chunk_count }}</div>
              </div>

              <div class="bento-stat-item">
                <div class="bento-stat-label">
                  <Icon icon="ri:shapes-line" class="w-3.5 h-3.5 text-state-success" />
                  <span>向量维度</span>
                </div>
                <div class="bento-stat-val font-mono">{{ status.embedding_dims || '—' }}</div>
              </div>

              <div class="bento-stat-item">
                <div class="bento-stat-label">
                  <Icon icon="ri:cpu-line" class="w-3.5 h-3.5 text-chart-5" />
                  <span>嵌入模型</span>
                </div>
                <div class="bento-stat-model" :title="status.embedding_model">
                  {{ status.embedding_model || '未配置' }}
                </div>
              </div>
            </div>

            <!-- 元信息列表 -->
            <div class="meta-list">
              <div class="meta-row">
                <span class="meta-k">最近构建时间</span>
                <span class="meta-v">{{ formatTime(status.last_built_at) }}</span>
              </div>
              <div class="meta-row">
                <span class="meta-k">检索匹配策略</span>
                <span class="meta-v">余弦相似度 Top-K</span>
              </div>
            </div>

            <!-- 错误信息栏 -->
            <div v-if="status.last_error" class="error-banner">
              <div class="flex items-center gap-1.5 text-state-error font-medium text-xs mb-1">
                <Icon icon="ri:error-warning-line" class="w-4 h-4 shrink-0" />
                <span>构建异常信息</span>
              </div>
              <div class="error-msg">{{ status.last_error }}</div>
            </div>
          </div>
        </div>

        <!-- 知识库说明卡 -->
        <div class="apple-card sidebar-card tips-card">
          <div class="flex items-center gap-2 mb-3">
            <Icon icon="ri:lightbulb-line" class="w-4 h-4 text-state-warning" />
            <h4 class="text-xs font-semibold text-foreground">使用说明与最佳实践</h4>
          </div>
          <ul class="tips-list">
            <li>
              <strong>公开内容索引</strong>：仅已发布（Published）状态的文章与单页会被向量化，草稿绝不入库。
            </li>
            <li>
              <strong>更新后重建</strong>：发布新文章或对正文进行大范围修改后，点击「重建知识库」即可同步。
            </li>
            <li>
              <strong>精准溯源</strong>：每次回答均附带对应文章来源卡片，点击可直接在浏览器中跳转查看。
            </li>
          </ul>
        </div>
      </aside>
    </div>
  </div>
</template>

<style scoped>
.assistant-page {
  display: flex;
  flex-direction: column;
  min-height: 100%;
}

.header-icon-box {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: 9px;
  background: color-mix(in srgb, var(--color-primary) 12%, transparent);
}

.assistant-layout {
  display: grid;
  grid-template-columns: 1fr;
  gap: 16px;
  padding: 16px;
  flex: 1;
}

@media (min-width: 1024px) {
  .assistant-layout {
    grid-template-columns: 1fr 340px;
    padding: 16px 20px;
  }
}

@media (min-width: 1280px) {
  .assistant-layout {
    grid-template-columns: 1fr 380px;
  }
}

/* ===== 左侧工作区 ===== */
.chat-workspace {
  display: flex;
  flex-direction: column;
  min-width: 0;
  height: calc(100vh - 150px);
  min-height: 580px;
}

.chat-card {
  display: flex;
  flex-direction: column;
  height: 100%;
  padding: 0;
  overflow: hidden;
  background:
    radial-gradient(120% 120% at 50% 0%, color-mix(in srgb, var(--color-primary) 4%, transparent) 0%, transparent 60%),
    color-mix(in srgb, var(--color-card) 85%, transparent);
  border: 1px solid color-mix(in srgb, #fff 8%, transparent);
  border-radius: 20px;
  box-shadow: 0 8px 32px -4px rgba(0, 0, 0, 0.4);
}

/* 消息滚动区 */
.chat-messages-scroll {
  flex: 1;
  overflow-y: auto;
  padding: 24px;
  scroll-behavior: smooth;
}

/* 欢迎页 */
.welcome-hero {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 100%;
  padding: 24px 12px;
  text-align: center;
}

.welcome-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 12px;
  border-radius: 99px;
  border: 1px solid color-mix(in srgb, var(--color-primary) 24%, transparent);
  background: color-mix(in srgb, var(--color-primary) 8%, transparent);
  color: var(--color-primary);
  font-size: 11px;
  font-weight: 500;
  margin-bottom: 16px;
}

.welcome-title {
  font-size: 22px;
  font-weight: 700;
  color: var(--color-foreground);
  letter-spacing: -0.02em;
  margin-bottom: 8px;
}

.welcome-subtitle {
  max-width: 520px;
  font-size: 13px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
  margin-bottom: 32px;
}

/* 推荐提示词网格 */
.suggested-grid {
  display: grid;
  grid-template-columns: repeat(1, minmax(0, 1fr));
  gap: 12px;
  width: 100%;
  max-width: 680px;
}

@media (min-width: 640px) {
  .suggested-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

.suggested-card {
  display: flex;
  flex-direction: column;
  text-align: left;
  padding: 14px 16px;
  border-radius: 14px;
  border: 1px solid color-mix(in srgb, #fff 8%, transparent);
  background: color-mix(in srgb, var(--color-card) 60%, transparent);
  transition: all var(--duration-fast, 150ms) var(--ease-standard);
  cursor: pointer;
}

.suggested-card:hover {
  border-color: color-mix(in srgb, var(--color-primary) 40%, transparent);
  background: color-mix(in srgb, var(--color-primary) 6%, transparent);
  transform: translateY(-2px);
}

.suggested-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.suggested-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 8px;
  background: color-mix(in srgb, var(--color-primary) 12%, transparent);
}

.suggested-tag {
  font-size: 10px;
  font-weight: 500;
  color: var(--color-muted-foreground);
  background: color-mix(in srgb, var(--color-muted-foreground) 10%, transparent);
  padding: 2px 6px;
  border-radius: 6px;
}

.suggested-card-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-foreground);
  margin-bottom: 4px;
}

.suggested-card-query {
  font-size: 11.5px;
  color: var(--color-muted-foreground);
  line-height: 1.4;
}

/* 消息流 */
.messages-list {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.message-row {
  display: flex;
  gap: 12px;
  max-width: 88%;
}

.message-row.is-user {
  margin-left: auto;
  flex-direction: row-reverse;
}

.message-row.is-assistant {
  margin-right: auto;
}

.message-avatar {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  flex: 0 0 auto;
  border-radius: 10px;
  margin-top: 2px;
}

.message-avatar.is-user {
  background: color-mix(in srgb, var(--color-primary) 20%, transparent);
  color: var(--color-primary);
  border: 1px solid color-mix(in srgb, var(--color-primary) 30%, transparent);
}

.message-avatar.is-assistant {
  background: color-mix(in srgb, #fff 10%, transparent);
  color: var(--color-foreground);
  border: 1px solid color-mix(in srgb, #fff 12%, transparent);
}

.message-bubble-wrap {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.message-row.is-user .message-bubble-wrap {
  align-items: flex-end;
}

.message-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
  font-size: 11px;
}

.message-author {
  font-weight: 600;
  color: var(--color-foreground);
}

.message-time {
  color: var(--color-muted-foreground);
  font-size: 10px;
}

.message-bubble {
  padding: 14px 18px;
  border-radius: 16px;
  font-size: 13.5px;
  line-height: 1.65;
  color: var(--color-foreground);
}

.message-row.is-user .message-bubble {
  background: color-mix(in srgb, var(--color-primary) 18%, transparent);
  border: 1px solid color-mix(in srgb, var(--color-primary) 32%, transparent);
  border-top-right-radius: 4px;
}

.message-row.is-assistant .message-bubble {
  background: color-mix(in srgb, var(--color-card) 90%, transparent);
  border: 1px solid color-mix(in srgb, #fff 10%, transparent);
  border-top-left-radius: 4px;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.25);
}

.message-bubble.is-error {
  border-color: color-mix(in srgb, var(--color-state-error) 40%, transparent);
  background: color-mix(in srgb, var(--color-state-error) 10%, transparent);
  color: var(--color-state-error);
}

.message-text {
  white-space: pre-wrap;
  word-break: break-word;
}

/* AI 富文本 Markdown 渲染样式 */
.ai-rendered-content {
  font-size: 13.5px;
  line-height: 1.7;
  color: var(--color-foreground, #f5f5f7);
  word-break: break-word;
}

.ai-rendered-content :deep(p) {
  margin: 0 0 10px 0;
}
.ai-rendered-content :deep(p:last-child) {
  margin-bottom: 0;
}

.ai-rendered-content :deep(.ai-heading) {
  font-weight: 650;
  color: #ffffff;
  letter-spacing: -0.01em;
  margin: 16px 0 8px 0;
}
.ai-rendered-content :deep(.ai-heading:first-child) {
  margin-top: 0;
}

.ai-rendered-content :deep(.ai-h1) {
  font-size: 16.5px;
  margin-bottom: 8px;
}
.ai-rendered-content :deep(.ai-h2) {
  font-size: 15px;
  padding-bottom: 4px;
  border-bottom: 1px solid color-mix(in srgb, var(--color-muted-foreground) 15%, transparent);
  margin-bottom: 8px;
}
.ai-rendered-content :deep(.ai-h3) {
  font-size: 14px;
  margin-bottom: 6px;
}

.ai-rendered-content :deep(strong) {
  color: #ffffff;
  font-weight: 600;
}

.ai-rendered-content :deep(.ai-inline-code) {
  font-family: var(--font-mono, monospace);
  font-size: 12px;
  padding: 1.5px 5.5px;
  border-radius: 5px;
  background: color-mix(in srgb, var(--color-primary, #2e8dff) 12%, color-mix(in srgb, #fff 6%, transparent));
  color: var(--color-primary-hover, #66abff);
  border: 1px solid color-mix(in srgb, var(--color-primary, #2e8dff) 25%, transparent);
}

.ai-rendered-content :deep(.ai-code-block) {
  margin: 10px 0;
  padding: 12px 14px;
  border-radius: 10px;
  background: rgba(0, 0, 0, 0.45);
  border: 1px solid color-mix(in srgb, #fff 10%, transparent);
  font-family: var(--font-mono, monospace);
  font-size: 12px;
  line-height: 1.5;
  overflow-x: auto;
  color: #e5e5ea;
}

.ai-rendered-content :deep(.ai-ul),
.ai-rendered-content :deep(.ai-ol) {
  margin: 6px 0 10px 0;
  padding-left: 20px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.ai-rendered-content :deep(.ai-ul) {
  list-style-type: disc;
}
.ai-rendered-content :deep(.ai-ol) {
  list-style-type: decimal;
}

.ai-rendered-content :deep(li) {
  line-height: 1.6;
}

.ai-rendered-content :deep(.ai-citation-pill) {
  display: inline-block;
  margin: 0 2px;
  padding: 0 5px;
  border-radius: 4px;
  font-family: var(--font-mono, monospace);
  font-size: 11px;
  font-weight: 600;
  color: var(--color-primary, #2e8dff);
  background: color-mix(in srgb, var(--color-primary, #2e8dff) 12%, transparent);
  border: 1px solid color-mix(in srgb, var(--color-primary, #2e8dff) 25%, transparent);
  line-height: 1.4;
}

.ai-rendered-content :deep(.ai-quote) {
  margin: 10px 0;
  padding: 8px 12px;
  border-left: 3px solid var(--color-primary, #2e8dff);
  background: color-mix(in srgb, var(--color-primary, #2e8dff) 8%, transparent);
  border-radius: 0 8px 8px 0;
  color: var(--color-muted-foreground, #8e8e93);
  font-size: 13px;
}

.ai-rendered-content :deep(.ai-link) {
  color: var(--color-primary, #2e8dff);
  text-decoration: underline;
  text-underline-offset: 2px;
}
.ai-rendered-content :deep(.ai-link:hover) {
  color: var(--color-primary-hover, #66abff);
}

/* 引用来源区 */
.citations-section {
  margin-top: 14px;
  padding-top: 12px;
  border-top: 1px solid color-mix(in srgb, var(--color-muted-foreground) 15%, transparent);
}

.citations-header {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11.5px;
  font-weight: 600;
  color: var(--color-muted-foreground);
  margin-bottom: 8px;
}

.citations-grid {
  display: grid;
  grid-template-columns: repeat(1, minmax(0, 1fr));
  gap: 6px;
}

@media (min-width: 640px) {
  .citations-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

.citation-card {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 10px;
  border-radius: 9px;
  border: 1px solid color-mix(in srgb, #fff 8%, transparent);
  background: color-mix(in srgb, var(--color-background) 60%, transparent);
  font-size: 12px;
  color: var(--color-foreground);
  text-decoration: none;
  transition: all var(--duration-fast, 150ms) ease;
}

.citation-card:hover {
  border-color: var(--color-primary);
  color: var(--color-primary);
  background: color-mix(in srgb, var(--color-primary) 8%, transparent);
}

.citation-badge {
  font-family: var(--font-mono, monospace);
  font-size: 10.5px;
  color: var(--color-primary);
  font-weight: 700;
}

.citation-title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.citation-link-icon {
  width: 13px;
  height: 13px;
  flex: 0 0 auto;
  color: var(--color-muted-foreground);
}

/* 消息操作 */
.message-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 6px;
}

.msg-action-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px 8px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--color-muted-foreground);
  font-size: 11px;
  cursor: pointer;
  transition: all var(--duration-fast, 150ms) ease;
}

.msg-action-btn:hover {
  color: var(--color-foreground);
  background: color-mix(in srgb, var(--color-muted-foreground) 12%, transparent);
}

.msg-action-btn.is-active {
  color: var(--color-state-success);
}

/* 思考中动画 */
.thinking-bubble {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 16px;
}

.thinking-dots {
  display: flex;
  gap: 4px;
}

.thinking-dots .dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--color-primary);
  animation: dot-pulse 1.4s infinite ease-in-out both;
}

.thinking-dots .dot:nth-child(1) { animation-delay: -0.32s; }
.thinking-dots .dot:nth-child(2) { animation-delay: -0.16s; }

@keyframes dot-pulse {
  0%, 80%, 100% { transform: scale(0.6); opacity: 0.4; }
  40% { transform: scale(1); opacity: 1; }
}

.thinking-text {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

/* 底部输入坞 */
.chat-input-dock {
  padding: 14px 18px 16px;
  border-top: 1px solid color-mix(in srgb, #fff 8%, transparent);
  background: color-mix(in srgb, var(--color-card) 95%, transparent);
  backdrop-filter: blur(20px);
}

.input-box-container {
  display: flex;
  flex-direction: column;
  border-radius: 14px;
  border: 1px solid color-mix(in srgb, #fff 12%, transparent);
  background: color-mix(in srgb, var(--color-background) 70%, transparent);
  transition: border-color var(--duration-fast, 150ms) ease, box-shadow var(--duration-fast, 150ms) ease;
}

.input-box-container:focus-within {
  border-color: var(--color-primary);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--color-primary) 20%, transparent);
}

.chat-textarea {
  width: 100%;
  padding: 12px 14px;
  background: transparent;
  border: 0;
  outline: none;
  font-size: 13.5px;
  line-height: 1.5;
  color: var(--color-foreground);
  resize: none;
}

.chat-textarea::placeholder {
  color: var(--color-muted-foreground);
  opacity: 0.7;
}

.input-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  border-top: 1px solid color-mix(in srgb, #fff 5%, transparent);
}

.input-hints {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 11px;
  color: var(--color-muted-foreground);
}

.kbd-hint kbd {
  display: inline-block;
  padding: 1px 4px;
  border-radius: 4px;
  font-size: 10px;
  font-family: var(--font-mono, monospace);
  background: color-mix(in srgb, var(--color-muted-foreground) 15%, transparent);
  color: var(--color-foreground);
  border: 1px solid color-mix(in srgb, #fff 10%, transparent);
}

.char-count {
  font-family: var(--font-mono, monospace);
  font-size: 10px;
}

/* ===== 右侧侧边栏 ===== */
.assistant-sidebar {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.sidebar-card {
  padding: 18px 20px;
  border-radius: 18px;
  background: color-mix(in srgb, var(--color-card) 85%, transparent);
  border: 1px solid color-mix(in srgb, #fff 8%, transparent);
}

.sidebar-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}

.sidebar-title {
  font-size: 14px;
  font-weight: 650;
  color: var(--color-foreground);
}

.sidebar-subtitle {
  font-size: 10.5px;
  color: var(--color-muted-foreground);
}

.status-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 3px 9px;
  border-radius: 99px;
  background: color-mix(in srgb, var(--color-muted-foreground) 10%, transparent);
  border: 1px solid color-mix(in srgb, #fff 6%, transparent);
}

.sidebar-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 36px 0;
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.building-progress-box {
  margin-bottom: 14px;
  padding: 10px 12px;
  border-radius: 12px;
  background: color-mix(in srgb, var(--color-primary) 8%, transparent);
  border: 1px solid color-mix(in srgb, var(--color-primary) 20%, transparent);
}

.progress-track {
  height: 5px;
  border-radius: 99px;
  background: color-mix(in srgb, var(--color-muted-foreground) 20%, transparent);
  overflow: hidden;
}

.progress-fill {
  height: 100%;
  border-radius: inherit;
  background: linear-gradient(90deg, var(--color-primary), var(--color-primary-hover));
  transition: width 300ms ease;
}

/* Bento 指标网格 */
.bento-stats-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  margin-bottom: 16px;
}

.bento-stat-item {
  display: flex;
  flex-direction: column;
  padding: 10px 12px;
  border-radius: 12px;
  background: color-mix(in srgb, var(--color-muted-foreground) 5%, transparent);
  border: 1px solid color-mix(in srgb, #fff 6%, transparent);
}

.bento-stat-label {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: 11px;
  color: var(--color-muted-foreground);
  margin-bottom: 4px;
}

.bento-stat-val {
  font-size: 18px;
  font-weight: 700;
  color: var(--color-foreground);
  letter-spacing: -0.02em;
}

.bento-stat-model {
  font-size: 11px;
  font-family: var(--font-mono, monospace);
  font-weight: 600;
  color: var(--color-foreground);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* 元数据列表 */
.meta-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding-top: 12px;
  border-top: 1px solid color-mix(in srgb, var(--color-muted-foreground) 12%, transparent);
}

.meta-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 11.5px;
}

.meta-k {
  color: var(--color-muted-foreground);
}

.meta-v {
  color: var(--color-foreground);
  font-weight: 500;
}

/* 错误警报 */
.error-banner {
  margin-top: 14px;
  padding: 10px 12px;
  border-radius: 10px;
  background: color-mix(in srgb, var(--color-state-error) 10%, transparent);
  border: 1px solid color-mix(in srgb, var(--color-state-error) 25%, transparent);
}

.error-msg {
  font-size: 11px;
  color: var(--color-state-error);
  line-height: 1.4;
  word-break: break-all;
}

/* 提示说明卡 */
.tips-card {
  background: color-mix(in srgb, var(--color-card) 60%, transparent);
}

.tips-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding-left: 0;
  list-style: none;
  font-size: 11.5px;
  line-height: 1.55;
  color: var(--color-muted-foreground);
}

.tips-list li strong {
  color: var(--color-foreground);
}
</style>
