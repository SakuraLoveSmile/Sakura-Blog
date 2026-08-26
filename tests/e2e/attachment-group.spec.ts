import { expect, test, type Page } from "@playwright/test";

const onePixelPng = Buffer.from(
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=",
  "base64",
);

async function login(page: Page) {
  await page.goto("/console/login");
  await page.getByPlaceholder("请输入用户名").fill("admin");
  await page.getByPlaceholder("请输入密码").fill("e2e-password");
  await page.getByRole("button", { name: "登 录", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/dashboard/, { timeout: 30_000 });
}

test("附件详情可以编辑所属分组且不再提示拖拽上传", async ({ page }) => {
  await login(page);
  await page.goto("/console/attachments");
  await expect(page.getByRole("heading", { name: "媒体库", exact: true })).toBeVisible();
  await expect(page.getByText("拖拽", { exact: false })).toHaveCount(0);
  await expect(page.getByText("松开鼠标即可上传文件", { exact: true })).toHaveCount(0);

  const groupName = `Playwright 附件组 ${Date.now()}`;
  const groupResponse = await page.request.post("/api/v1/attachment-groups", { data: { name: groupName } });
  expect(groupResponse.ok()).toBeTruthy();
  const group = (await groupResponse.json()) as { id: number };

  const fileName = `playwright-attachment-${Date.now()}.png`;
  const uploadResponsePromise = page.waitForResponse(
    (response) => response.url().includes("/api/v1/attachments/upload") && response.request().method() === "POST",
  );
  const chooserPromise = page.waitForEvent("filechooser");
  await page.getByRole("button", { name: "上传附件", exact: true }).click();
  await (await chooserPromise).setFiles({ name: fileName, mimeType: "image/png", buffer: onePixelPng });
  expect((await uploadResponsePromise).ok()).toBeTruthy();
  await expect(page.getByText(fileName, { exact: true })).toBeVisible({ timeout: 15_000 });

  await page.getByText(fileName, { exact: true }).click();
  await expect(page.getByRole("heading", { name: "附件属性详情", exact: true })).toBeVisible();

  const groupSelect = page.getByTestId("attachment-detail-group");
  await expect(groupSelect).toHaveValue("0");
  await groupSelect.selectOption(String(group.id));
  await expect(page.getByTestId("save-attachment-group")).toBeEnabled();

  const moveResponsePromise = page.waitForResponse(
    (response) => response.url().includes("/api/v1/attachments/move") && response.request().method() === "POST",
  );
  await page.getByTestId("save-attachment-group").click();
  expect((await moveResponsePromise).ok()).toBeTruthy();
  await expect(page.getByText("附件分组已更新", { exact: true })).toBeVisible();
  await expect(groupSelect).toHaveValue(String(group.id));

  await page.getByRole("button", { name: "关闭", exact: true }).click();
  await page.locator(".group-item-btn").filter({ hasText: groupName }).click();
  await expect(page.getByText(fileName, { exact: true })).toBeVisible();
});
