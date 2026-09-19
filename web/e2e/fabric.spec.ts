import { test, expect } from "@playwright/test";
import path from "node:path";
import fs from "node:fs";
test("mobile photo, dimensions, remnants, history, trash and restore", async ({
  page,
}) => {
  await page.setViewportSize({ width: 375, height: 667 });
  await page.goto("./");
  await expect(
    page.getByRole("heading", { name: "我的布料", exact: true }),
  ).toBeVisible();
  await page.getByRole("link", { name: "新增布料", exact: true }).click();
  await page.getByLabel("布料名称").fill("测试 · 蓝色棉麻 " + Date.now());
  await page.getByRole("button", { name: "棉", exact: true }).click();
  await page.getByRole("button", { name: "麻", exact: true }).click();
  await page.getByLabel("布片 1 幅宽", { exact: true }).fill("150");
  await page.getByLabel("布片 1 长度", { exact: true }).fill("250");
  await page.getByLabel("收纳位置").fill("测试收纳箱");
  const sample = path.resolve("../.local/fixtures/example.heic");
  if (fs.existsSync(sample)) {
    await page.getByLabel("选择照片文件").setInputFiles(sample);
  } else {
    const data = await page.evaluate(() => {
      const c = document.createElement("canvas");
      c.width = 400;
      c.height = 300;
      const x = c.getContext("2d")!;
      x.fillStyle = "#608378";
      x.fillRect(0, 0, 400, 300);
      return c.toDataURL("image/jpeg").split(",")[1]!;
    });
    await page
      .getByLabel("选择照片文件")
      .setInputFiles({
        name: "fabric.jpg",
        mimeType: "image/jpeg",
        buffer: Buffer.from(data, "base64"),
      });
  }
  await expect(page.getByText("1 / 10", { exact: true })).toBeVisible({
    timeout: 30000,
  });
  await page.getByRole("button", { name: "保存布料", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: /测试 · 蓝色棉麻/ }),
  ).toBeVisible();
  await expect(page.getByText("150 × 250 cm", { exact: true })).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  const detail = page.url();
  await page.screenshot({
    path: "../.local/mobile-detail.png",
    fullPage: true,
  });
  await page.getByRole("link", { name: "更新余料", exact: true }).click();
  await page.getByLabel("布片 1 长度", { exact: true }).fill("180");
  await page.getByRole("button", { name: "保存修改", exact: true }).click();
  await expect(page.getByText("150 × 180 cm", { exact: true })).toBeVisible();
  await expect(page.getByText("使用中", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "查看修改历史" }).click();
  await expect(page.getByText("首次记录", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "删除布料", exact: true }).click();
  await page.getByRole("button", { name: "移入回收站", exact: true }).click();
  await page.getByRole("link", { name: "回收站", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: /测试 · 蓝色棉麻/ }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "恢复布料", exact: true })
    .first()
    .click();
  await page.goto(detail);
  await expect(page.getByText("150 × 180 cm", { exact: true })).toBeVisible();
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto("./");
  await page.screenshot({
    path: "../.local/desktop-library.png",
    fullPage: true,
  });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
});
test("narrow layouts and camera selection", async ({ page }) => {
  for (const width of [320, 375, 390, 768, 1440]) {
    await page.setViewportSize({ width, height: width === 1440 ? 900 : 667 });
    await page.goto("./new");
    await expect(
      page.getByRole("heading", { name: "记录新布料", exact: true }),
    ).toBeVisible();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    await expect(page.getByLabel("拍照上传")).toHaveAttribute(
      "capture",
      "environment",
    );
    await expect(page.getByLabel("选择照片文件")).toHaveAttribute(
      "multiple",
      "",
    );
    await page.getByText("更多细节", { exact: true }).click();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    if (width === 375)
      await page.screenshot({
        path: "../.local/mobile-editor.png",
        fullPage: true,
      });
  }
});
