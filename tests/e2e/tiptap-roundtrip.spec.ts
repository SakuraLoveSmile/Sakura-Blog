import { expect, test } from "@playwright/test";

const goldenCases = [
  {
    name: "rich text",
    markdown: "# Heading\n\nA **bold** and *italic* [link](https://example.com).",
  },
  {
    name: "table",
    markdown: "| Name | Value |\n| --- | --- |\n| alpha | 1 |\n| beta | 2 |",
  },
  {
    name: "task list",
    // TipTap 2 canonicalizes adjacent task items with a blank line.
    markdown: "- [x] shipped\n\n- [ ] pending",
  },
  {
    name: "code block",
    markdown: '```go\nfmt.Println("hello")\n```',
  },
  {
    name: "Stellar block tags",
    markdown:
      "{% tip text:提示 %}\n保留 **Markdown**。\n{% endtip %}\n\n{% table style:scroll %}\n| A | B |\n| --- | --- |\n| 1 | 2 |\n{% endtable %}",
  },
];

test("TipTap 2 golden round-trip baseline has zero normalized diff", async ({ page }) => {
  await page.goto("/console/login");
  await page.getByPlaceholder("请输入用户名").fill("admin");
  await page.getByPlaceholder("请输入密码").fill("e2e-password");
  await page.getByRole("button", { name: "登 录", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/dashboard/, { timeout: 30_000 });
  await page.goto("/console/posts/new");
  await page.locator('[contenteditable="true"]').first().waitFor({ state: "visible", timeoutMs: 15_000 });

  for (const golden of goldenCases) {
    const roundTrip = await page.evaluate((markdown) => {
      const editor = (
        window as Window & {
          __consoleTiptapEditor?: {
            commands: { setContent: (value: string, emitUpdate?: boolean) => void };
            storage: { markdown: { getMarkdown: () => string } };
          };
        }
      ).__consoleTiptapEditor;
      if (!editor) throw new Error("development TipTap editor hook is unavailable");
      editor.commands.setContent(markdown, false);
      return editor.storage.markdown.getMarkdown();
    }, golden.markdown);
    const normalize = (value: string) => value.replace(/\r\n/g, "\n").trimEnd();
    expect(normalize(roundTrip), golden.name).toBe(normalize(golden.markdown));
  }
});
