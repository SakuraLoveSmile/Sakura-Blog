import type { FixtureItem } from "./fixture-types";

export const FIXTURE_ITEMS: FixtureItem[] = [
  {
    id: "friends",
    title: "友链卡片 (Friends)",
    category: "数据服务",
    endpoint: "/api/v1/preview/friends",
    format: "json",
    description: "为 {% friends %} 标签提供本地确定的头像、博客标题与链接列表数据，支持普通卡片、专辑封面与海报视图。",
    icon: "ri:user-shared-line",
    previewShortcode: "{% friends %}",
    previewHeight: 280,
    sampleShortcodes: ["{% friends %}", "{% friends type:card %}", "{% friends type:list %}"],
    defaultData: JSON.stringify(
      {
        content: [
          {
            title: "Fixture 博客",
            url: "https://example.com",
            html_url: "https://example.com",
            avatar_url:
              "data:image/svg+xml;charset=utf-8,%3Csvg%20xmlns='http://www.w3.org/2000/svg'%20width='96'%20height='96'%3E%3Crect%20width='96'%20height='96'%20rx='48'%20fill='%234a6cf7'/%3E%3Ctext%20x='48'%20y='62'%20font-size='40'%20text-anchor='middle'%20fill='white'%3EB%3C/text%3E%3C/svg%3E",
            labels: [],
          },
          {
            title: "Fixture Docs",
            url: "https://docs.example.com",
            html_url: "https://docs.example.com",
            avatar_url:
              "data:image/svg+xml;charset=utf-8,%3Csvg%20xmlns='http://www.w3.org/2000/svg'%20width='96'%20height='96'%3E%3Crect%20width='96'%20height='96'%20rx='48'%20fill='%234a6cf7'/%3E%3Ctext%20x='48'%20y='62'%20font-size='40'%20text-anchor='middle'%20fill='white'%3ED%3C/text%3E%3C/svg%3E",
            labels: [],
          },
          {
            title: "Alice 的小站",
            url: "https://alice.example.com",
            html_url: "https://alice.example.com",
            avatar_url:
              "data:image/svg+xml;charset=utf-8,%3Csvg%20xmlns='http://www.w3.org/2000/svg'%20width='96'%20height='96'%3E%3Crect%20width='96'%20height='96'%20rx='48'%20fill='%234a6cf7'/%3E%3Ctext%20x='48'%20y='62'%20font-size='40'%20text-anchor='middle'%20fill='white'%3EA%3C/text%3E%3C/svg%3E",
            labels: [],
          },
          {
            title: "Bob 实验室",
            url: "https://bob.example.com",
            html_url: "https://bob.example.com",
            avatar_url:
              "data:image/svg+xml;charset=utf-8,%3Csvg%20xmlns='http://www.w3.org/2000/svg'%20width='96'%20height='96'%3E%3Crect%20width='96'%20height='96'%20rx='48'%20fill='%234a6cf7'/%3E%3Ctext%20x='48'%20y='62'%20font-size='40'%20text-anchor='middle'%20fill='white'%3EB%3C/text%3E%3C/svg%3E",
            labels: [],
          },
        ],
      },
      null,
      2,
    ),
    emptyData: JSON.stringify({ content: [] }, null, 2),
    schemaHint: "{ content: Array<{ title: string, url: string, ... }> }",
  },
  {
    id: "sites",
    title: "站点导航 (Sites)",
    category: "数据服务",
    endpoint: "/api/v1/preview/sites",
    format: "json",
    description: "为 {% sites %} 标签提供多列站点卡片数据（含封面图、图标、描述与彩色标签）。",
    icon: "ri:compass-3-line",
    previewShortcode: "{% sites %}",
    previewHeight: 300,
    sampleShortcodes: ["{% sites %}"],
    defaultData: JSON.stringify(
      {
        content: [
          {
            title: "Fixture Search",
            url: "https://search.example.com",
            icon: "data:image/svg+xml;charset=utf-8,%3Csvg%20xmlns='http://www.w3.org/2000/svg'%20width='96'%20height='96'%3E%3Crect%20width='96'%20height='96'%20rx='48'%20fill='%234a6cf7'/%3E%3Ctext%20x='48'%20y='62'%20font-size='40'%20text-anchor='middle'%20fill='white'%3ES%3C/text%3E%3C/svg%3E",
            cover: "/assets/img/preview/banner-bg.jpg",
            description: "确定性示例搜索引擎与快捷导航服务",
            labels: [
              {
                name: "工具",
                color: "4a6cf7",
                lightness: 40,
                hue: 228,
                saturation: 90,
              },
            ],
          },
          {
            title: "Fixture Docs",
            url: "https://docs.example.com",
            icon: "data:image/svg+xml;charset=utf-8,%3Csvg%20xmlns='http://www.w3.org/2000/svg'%20width='96'%20height='96'%3E%3Crect%20width='96'%20height='96'%20rx='48'%20fill='%234a6cf7'/%3E%3Ctext%20x='48'%20y='62'%20font-size='40'%20text-anchor='middle'%20fill='white'%3ED%3C/text%3E%3C/svg%3E",
            cover: "/assets/img/preview/gallery-1.jpg",
            description: "示例文档中心与开发者 API 指南",
            labels: [
              {
                name: "文档",
                color: "10b981",
                lightness: 40,
                hue: 152,
                saturation: 88,
              },
            ],
          },
          {
            title: "Fixture News",
            url: "https://news.example.com",
            icon: "data:image/svg+xml;charset=utf-8,%3Csvg%20xmlns='http://www.w3.org/2000/svg'%20width='96'%20height='96'%3E%3Crect%20width='96'%20height='96'%20rx='48'%20fill='%234a6cf7'/%3E%3Ctext%20x='48'%20y='62'%20font-size='40'%20text-anchor='middle'%20fill='white'%3EN%3C/text%3E%3C/svg%3E",
            cover: "/assets/img/preview/gallery-2.jpg",
            description: "示例资讯站与前沿技术周刊",
            labels: [
              {
                name: "资讯",
                color: "f59e0b",
                lightness: 48,
                hue: 38,
                saturation: 92,
              },
            ],
          },
        ],
      },
      null,
      2,
    ),
    emptyData: JSON.stringify({ content: [] }, null, 2),
    schemaHint: "{ content: Array<{ title: string, url: string, ... }> }",
  },
  {
    id: "rating",
    title: "文章评分 (Rating)",
    category: "交互组件",
    endpoint: "/api/v1/preview/rating",
    format: "json",
    description: "为 {% rating %} 交互短代码提供只读初始评分（平均分 avg、总评次数 count）。",
    icon: "ri:star-line",
    previewShortcode: "{% rating %}",
    previewHeight: 200,
    sampleShortcodes: ["{% rating %}", "{% rating id:custom-post %}"],
    defaultData: JSON.stringify(
      {
        avg: 4.6,
        count: 128,
      },
      null,
      2,
    ),
    emptyData: JSON.stringify({ avg: 0, count: 0 }, null, 2),
    schemaHint: "{ avg: number, count: number >= 0 }",
  },
  {
    id: "vote",
    title: "文章投票 (Vote)",
    category: "交互组件",
    endpoint: "/api/v1/preview/vote/info",
    format: "json",
    description: "为 {% vote %} 交互短代码提供点赞与踩计数值（up / down）。",
    icon: "ri:thumb-up-line",
    previewShortcode: "{% vote %}",
    previewHeight: 200,
    sampleShortcodes: ["{% vote %}", "{% vote id:demo-topic %}"],
    defaultData: JSON.stringify(
      {
        votes: {
          up: 86,
          down: 12,
        },
      },
      null,
      2,
    ),
    emptyData: JSON.stringify({ votes: { up: 0, down: 0 } }, null, 2),
    schemaHint: "{ votes: { up: number >= 0, down: number >= 0 } }",
  },
  {
    id: "md",
    title: "外部 Markdown (MD)",
    category: "外部文档",
    endpoint: "/api/v1/preview/md",
    format: "markdown",
    description: "为 {% md src:... %} 标签提供外部拉取的 Markdown 正文内容与代码块。",
    icon: "ri:markdown-line",
    previewShortcode: "{% md src:/assets/md/preview/example.md %}",
    previewHeight: 300,
    sampleShortcodes: ["{% md src:/assets/md/preview/example.md %}"],
    emptyData: "",
    schemaHint: "Markdown text",
    defaultData: `# 外部 Markdown Fixture

这是一段**确定性 Fixture** 内容，用于组件库预览：

- 不依赖公网接口；
- 结构与真实外部 Markdown 一致；
- 渲染失败时显示降级提示。

\`\`\`go
func fixture() string {
    return "preview"
}
\`\`\`
`,
  },
];
