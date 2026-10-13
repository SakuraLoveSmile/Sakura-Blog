// Blog 本地能力分别由独立 manifest entry 声明，以 region 为资源所有者。
const DEFINITIONS = {
  'blog-comments': { asset: '/assets/js/comments.js', global: () => window.stellar.comments },
  'blog-ai-summary': { asset: '/assets/js/ai-summary.js', global: () => window.blogAISummary },
  'blog-ai-ask': { asset: '/assets/js/search/ai-ask.js', global: () => window.blogAIAsk },
  'blog-checkbox': { asset: '/assets/js/plugins/checkbox-interactive.js', global: () => window.blogCheckbox },
  'blog-tagcloud': { asset: '/assets/js/tagcloud.js', global: () => window.blogTagcloud },
  'blog-reading': { asset: '/assets/js/reading.js', global: () => window.blogReading }
};
export async function mount(root, context) {
  const definition = DEFINITIONS[context.extension.id];
  if (!definition) throw new TypeError('unknown Blog native extension');
  await context.assets.script('/assets/js/blog/lifecycle.js');
  await context.assets.script(definition.asset);
  context.signal.throwIfAborted();
  return definition.global().mount(root, context);
}
