import { test, expect } from "@playwright/test";

for (const width of [320, 375, 1440]) {
  test("photo preview while creating and editing at " + width + "px", async ({ page }) => {
    await page.setViewportSize({ width, height: width === 1440 ? 900 : 667 });
    const errors: string[] = [];
    page.on("pageerror", error => errors.push(error.message));
    await page.goto("./new");
    const name = "大图预览测试 " + width + " " + Date.now();
    await page.getByLabel("布料名称", { exact: true }).fill(name);
    const data = await page.evaluate(() => {
      const canvas = document.createElement("canvas");
      canvas.width = 1200; canvas.height = 800;
      const ctx = canvas.getContext("2d")!;
      ctx.fillStyle = "#386a56"; ctx.fillRect(0, 0, 1200, 800);
      ctx.fillStyle = "#f0d9ae";
      for (let x = 0; x < 1200; x += 60) ctx.fillRect(x, 0, 12, 800);
      return canvas.toDataURL("image/jpeg").split(",")[1]!;
    });
    let release!: () => void;
    const gate = new Promise<void>(resolve => { release = resolve; });
    await page.route("**/api/uploads", async route => { await gate; await route.continue(); });
    const writes: string[] = [];
    page.on("request", request => {
      if (["POST", "PUT"].includes(request.method()) && /\/api\/fabrics(?:\/[^/]+)?$/.test(request.url())) writes.push(request.method());
    });
    const sample = { name: "preview.jpg", mimeType: "image/jpeg", buffer: Buffer.from(data, "base64") };
    await page.getByLabel("选择照片文件").setInputFiles(sample);
    await page.getByRole("button", { name: "查看待上传照片 preview.jpg 大图", exact: true }).click();
    const modal = page.getByRole("dialog", { name: "照片预览", exact: true });
    const image = modal.locator("img");
    await expect(modal).toBeVisible();
    await expect(image).toHaveAttribute("src", /^blob:/);
    await expect.poll(() => image.evaluate((img: HTMLImageElement) => img.complete && img.naturalWidth > 0)).toBe(true);
    release();
    await expect(page.getByText("1 / 10", { exact: true })).toBeVisible();
    await expect(image).toHaveAttribute("src", /\/media\/[a-f0-9]{32}\/main$/);
    await expect.poll(() => image.evaluate((img: HTMLImageElement) => img.complete && img.naturalWidth > 0)).toBe(true);
    const bounds = await image.evaluate(img => {
      const rect = img.getBoundingClientRect();
      return rect.left >= 0 && rect.right <= innerWidth && rect.top >= 0 && rect.bottom <= innerHeight;
    });
    expect(bounds).toBe(true);
    await page.screenshot({ path: "../.local/photo-preview-" + width + ".png" });
    await modal.getByRole("button", { name: "关闭", exact: true }).click();
    await expect(modal).toHaveCount(0);
    await expect(page.getByLabel("布料名称", { exact: true })).toHaveValue(name);
    const trigger = page.getByRole("button", { name: "查看照片 1 大图", exact: true });
    await trigger.focus(); await page.keyboard.press("Enter");
    await expect(modal).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(modal).toHaveCount(0);
    await expect(trigger).toBeFocused();
    await trigger.click();
    await page.locator(".modal-scrim").click({ position: { x: 2, y: 2 } });
    await expect(modal).toHaveCount(0);
    expect(writes).toEqual([]);
    await page.getByRole("button", { name: "保存布料", exact: true }).click();
    await expect(page.getByRole("heading", { name, exact: true })).toBeVisible();
    await page.getByRole("link", { name: "编辑资料", exact: true }).click();
    await trigger.click();
    await expect(modal).toBeVisible();
    await expect(image).toHaveAttribute("src", /\/main$/);
    await expect.poll(() => image.evaluate((img: HTMLImageElement) => img.complete && img.naturalWidth > 0)).toBe(true);
    await modal.getByRole("button", { name: "关闭", exact: true }).click();
    await expect(page.getByLabel("布料名称", { exact: true })).toHaveValue(name);
    expect(writes).toEqual(["POST"]);
    // Failed uploads remain previewable; retry and remove controls keep their own actions.
    await page.unroute("**/api/uploads");
    await page.route("**/api/uploads", route => route.fulfill({ status: 503, contentType: "application/json", body: '{"code":"busy","message":"测试上传失败"}' }));
    await page.getByLabel("选择照片文件").setInputFiles(sample);
    await expect(page.getByRole("button", { name: "重试", exact: true })).toBeVisible();
    // The retry control overlays the center; click the exposed photo area.
    await page.getByRole("button", { name: "查看待上传照片 preview.jpg 大图", exact: true }).click({ position: { x: 10, y: 10 } });
    await expect(image).toHaveAttribute("src", /^blob:/);
    await modal.getByRole("button", { name: "关闭", exact: true }).click();
    const retried = page.waitForResponse(response => response.url().endsWith("/api/uploads") && response.status() === 503);
    await page.getByRole("button", { name: "重试", exact: true }).click();
    await retried;
    await expect(page.getByRole("button", { name: "重试", exact: true })).toBeVisible();
    await expect(modal).toHaveCount(0);
    await page.getByRole("button", { name: "移除待上传照片", exact: true }).click();
    await expect(modal).toHaveCount(0);
    await expect(page.getByText("1 / 10", { exact: true })).toBeVisible();
    await page.getByRole("button", { name: "移除照片 1", exact: true }).click();
    await expect(trigger).toHaveCount(0);
    await expect(modal).toHaveCount(0);
    expect(writes).toEqual(["POST"]);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    expect(errors).toEqual([]);
  });
}
