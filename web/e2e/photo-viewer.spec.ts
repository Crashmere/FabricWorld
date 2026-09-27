import { test, expect, type Page } from "@playwright/test";
import { randomBytes } from "node:crypto";
import { newFabric } from "../src/types";

// Use desktop input for native mouse-wheel checks, including narrow viewports.
// The pinch case below creates its own mobile touch context.
test.use({ isMobile: false });

async function createPhotos(page: Page) {
  await page.goto("./");
  const photoIds: string[] = [];
  for (const [width, height] of [[1200, 800], [800, 1200]]) {
    const base64 = await page.evaluate(({ width, height }) => {
      const canvas = document.createElement("canvas");
      canvas.width = width!; canvas.height = height!;
      const ctx = canvas.getContext("2d")!;
      ctx.fillStyle = "#386a56"; ctx.fillRect(0, 0, width!, height!);
      ctx.fillStyle = "#f0d9ae";
      for (let x = 0; x < width!; x += 60) ctx.fillRect(x, 0, 12, height!);
      return canvas.toDataURL("image/jpeg").split(",")[1]!;
    }, { width, height });
    const uploaded = await page.request.post("./api/uploads", {
      headers: { "Idempotency-Key": randomBytes(16).toString("hex") },
      multipart: { file: { name: "zoom.jpg", mimeType: "image/jpeg", buffer: Buffer.from(base64, "base64") } },
    });
    expect(uploaded.ok()).toBe(true);
    photoIds.push((await uploaded.json()).id);
  }
  const created = await page.request.post("./api/fabrics", {
    headers: { "Idempotency-Key": randomBytes(16).toString("hex") },
    data: { ...newFabric(), name: "图片缩放测试 " + Date.now(), photoIds },
  });
  expect(created.ok()).toBe(true);
  const fabric = await created.json();
  await page.goto("./fabrics/" + fabric.id);
  await page.getByRole("button", { name: "查看照片大图", exact: true }).click();
  await expect(page.getByRole("button", { name: "放大图片", exact: true })).toBeEnabled();
}

for (const width of [320, 375, 1440]) {
  test("large photo viewer zoom, pan, fit and navigation at " + width + "px", async ({ page }) => {
    await page.setViewportSize({ width, height: width === 1440 ? 900 : 667 });
    const errors: string[] = [];
    page.on("pageerror", error => errors.push(error.message));
    await createPhotos(page);
    const writes: string[] = [];
    page.on("request", request => {
      if (request.url().includes("/api/") && request.method() !== "GET") writes.push(request.method());
    });
    const modal = page.getByRole("dialog");
    const stage = page.getByRole("region", { name: "图片缩放区域", exact: true });
    const image = modal.locator("img");
    const scale = modal.getByLabel("图片缩放比例", { exact: true });
    const fit = modal.getByRole("button", { name: "适应窗口", exact: true });
    const browserScale = await page.evaluate(() => visualViewport!.scale);
    const fitted = await image.boundingBox();
    const frame = await modal.boundingBox();
    expect(frame!.width).toBeGreaterThan(width * 0.95);
    expect(frame!.height).toBeGreaterThan((width === 1440 ? 900 : 667) * 0.95);
    await expect(scale).toHaveText("100%");
    await expect(modal.getByRole("button", { name: "缩小图片", exact: true })).toBeDisabled();
    await modal.getByRole("button", { name: "放大图片", exact: true }).click();
    await expect(scale).toHaveText("150%");
    expect((await image.boundingBox())!.width).toBeCloseTo(fitted!.width * 1.5, 0);
    const beforePan = await image.boundingBox();
    const bounds = (await stage.boundingBox())!;
    const center = { x: bounds.x + bounds.width / 2, y: bounds.y + bounds.height / 2 };
    await page.mouse.move(center.x, center.y);
    await page.mouse.down();
    await page.mouse.move(center.x + 70, center.y + 40, { steps: 5 });
    await page.mouse.up();
    expect((await image.boundingBox())!.x).toBeGreaterThan(beforePan!.x + 20);
    await page.mouse.wheel(0, -80);
    await expect.poll(async () => Number((await scale.textContent())!.replace("%", ""))).toBeGreaterThan(150);
    await fit.click();
    await expect(scale).toHaveText("100%");
    await stage.dblclick();
    await expect(scale).toHaveText("200%");
    await stage.focus();
    await page.keyboard.press("-");
    await expect(scale).toHaveText("133%");
    await page.keyboard.press("0");
    await expect(scale).toHaveText("100%");
    for (let i = 0; i < 5; i++) await modal.getByRole("button", { name: "放大图片", exact: true }).click();
    await expect(scale).toHaveText("600%");
    await expect(modal.getByRole("button", { name: "放大图片", exact: true })).toBeDisabled();
    await modal.getByRole("button", { name: "下一张", exact: true }).click();
    await expect(scale).toHaveText("100%");
    await expect(modal.getByRole("button", { name: "放大图片", exact: true })).toBeEnabled();
    expect(await image.evaluate(img => img.getBoundingClientRect().height > img.getBoundingClientRect().width)).toBe(true);
    await expect(modal.getByRole("button", { name: "下一张", exact: true })).toBeDisabled();
    await modal.getByRole("button", { name: "放大图片", exact: true }).click();
    await page.setViewportSize({ width, height: width === 1440 ? 850 : 600 });
    await expect(scale).toHaveText("100%");
    await page.setViewportSize({ width, height: width === 1440 ? 900 : 667 });
    // Focus wraps across only the enabled controls, including the image region.
    await modal.getByRole("button", { name: "关闭", exact: true }).focus();
    await page.keyboard.press("Shift+Tab");
    await expect(modal.getByRole("button", { name: "上一张", exact: true })).toBeFocused();
    await page.keyboard.press("Tab");
    await expect(modal.getByRole("button", { name: "关闭", exact: true })).toBeFocused();
    await page.screenshot({ path: "../.local/photo-viewer-" + test.info().project.name + "-" + width + ".png" });
    expect(await page.evaluate(() => visualViewport!.scale)).toBe(browserScale);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.keyboard.press("Escape");
    await expect(modal).toHaveCount(0);
    await expect(page.getByRole("button", { name: "查看照片大图", exact: true })).toBeFocused();
    await page.route("**/media/**/main", route => route.abort());
    // Start a new document: WebKit can reuse an already decoded image in-page.
    await page.reload();
    await page.getByRole("button", { name: "查看照片大图", exact: true }).click();
    await expect(modal.getByText("照片加载失败，请关闭后重新打开。", { exact: true })).toBeVisible();
    await expect(modal.getByRole("button", { name: "放大图片", exact: true })).toBeDisabled();
    await modal.getByRole("button", { name: "关闭", exact: true }).click();
    await page.unroute("**/media/**/main");
    await page.reload();
    await page.getByRole("button", { name: "查看照片大图", exact: true }).click();
    await expect(modal.getByRole("button", { name: "放大图片", exact: true })).toBeEnabled();
    await expect(scale).toHaveText("100%");
    await modal.getByRole("button", { name: "关闭", exact: true }).click();
    expect(writes).toEqual([]);
    expect(errors).toEqual([]);
  });
}

test("two-finger pinch scales the image without zooming the page", async ({ browser, browserName, baseURL }) => {
  test.skip(browserName !== "chromium", "Native multi-touch injection uses Chromium CDP; WebKit covers controls and mouse gestures.");
  const context = await browser.newContext({ baseURL, viewport: { width: 375, height: 667 }, isMobile: true, hasTouch: true });
  const page = await context.newPage();
  try {
    await createPhotos(page);
    const session = await context.newCDPSession(page);
    const stage = page.getByRole("region", { name: "图片缩放区域", exact: true });
    const bounds = (await stage.boundingBox())!;
    const cx = bounds.x + bounds.width / 2, cy = bounds.y + bounds.height / 2;
    const touch = (id: number, x: number, y: number) => ({ id, x, y });
    const scale = page.getByLabel("图片缩放比例", { exact: true });
    const pageScale = await page.evaluate(() => visualViewport!.scale);
    await session.send("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [touch(1, cx - 30, cy), touch(2, cx + 30, cy)] });
    for (const distance of [40, 50, 60, 75, 90]) {
      await session.send("Input.dispatchTouchEvent", { type: "touchMove", touchPoints: [touch(1, cx - distance, cy), touch(2, cx + distance, cy)] });
    }
    await expect.poll(async () => Number((await scale.textContent())!.replace("%", ""))).toBeGreaterThan(200);
    // CDP touchEnd lists the released contact, leaving the first finger down.
    await session.send("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [touch(2, cx + 90, cy)] });
    const before = await page.locator(".photo-viewer-image").boundingBox();
    await session.send("Input.dispatchTouchEvent", { type: "touchMove", touchPoints: [touch(1, cx - 50, cy + 40)] });
    await session.send("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
    expect((await page.locator(".photo-viewer-image").boundingBox())!.x).toBeGreaterThan(before!.x + 10);
    expect(await page.evaluate(() => visualViewport!.scale)).toBe(pageScale);
    await page.getByRole("button", { name: "适应窗口", exact: true }).click();
    await expect(scale).toHaveText("100%");
    await expect(stage).not.toHaveClass(/dragging/);
  } finally { await context.close(); }
});
