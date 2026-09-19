import { test, expect } from "@playwright/test";
import path from "node:path";
import fs from "node:fs";
import { randomBytes } from "node:crypto";
import { newFabric, newPiece } from "../src/types";
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
    await page.getByLabel("选择照片文件").setInputFiles({
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
  await expect(page.getByLabel("布料名称")).toHaveCount(0);
  await expect(page.getByLabel("选择照片文件")).toHaveCount(0);
  await page.getByLabel("布片 1 长度", { exact: true }).fill("180");
  await page.getByRole("button", { name: "保存余料", exact: true }).click();
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
test("details correction and focused remnant flow", async ({ page }) => {
  const input = {
    ...newFabric(),
    name: "余料简化 " + Date.now(),
    materials: ["棉"],
    location: "收纳箱",
    price: "35.80",
    notes: "保留原资料",
    pieces: [{ ...newPiece(), width: "150", length: "250" }],
  };
  const created = await page.request.post("./api/fabrics", {
    data: input,
    headers: { "Idempotency-Key": randomBytes(16).toString("hex") },
  });
  expect(created.ok()).toBe(true);
  const original = await created.json();
  const detail = "./fabrics/" + original.id;
  await page.setViewportSize({ width: 375, height: 667 });
  await page.goto(detail);
  await page.getByRole("link", { name: "编辑资料", exact: true }).click();
  await expect(
    page.getByLabel("布片 1 长度", { exact: true }),
  ).not.toBeVisible();
  await page.getByText("修正尺寸或状态", { exact: true }).click();
  await page.getByLabel("布片 1 长度", { exact: true }).fill("240");
  await page.getByRole("button", { name: "保存修改", exact: true }).click();
  await expect(page.getByText("未使用", { exact: true })).toBeVisible();
  await expect(page.getByText("150 × 240 cm", { exact: true })).toBeVisible();
  await page.goto(detail + "/edit?mode=remnant");
  await expect(page).toHaveURL(new RegExp("/" + original.id + "/remnant$"));
  await expect(
    page.getByRole("heading", { name: "更新余料", exact: true }),
  ).toBeVisible();
  await expect(page.getByLabel("布料名称")).toHaveCount(0);
  await expect(page.getByLabel("选择照片文件")).toHaveCount(0);
  await expect(page.getByLabel("购买日期")).toHaveCount(0);
  for (const width of [320, 375, 1440]) {
    await page.setViewportSize({ width, height: width === 1440 ? 900 : 667 });
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    await expect(
      page.getByRole("button", { name: "保存余料", exact: true }),
    ).toBeVisible();
    if (width === 375 || width === 1440)
      await page.screenshot({
        path: `../.local/remnant-${width}.png`,
        fullPage: true,
      });
  }
  await page.setViewportSize({ width: 375, height: 667 });
  await page.getByLabel("布片 1 长度", { exact: true }).fill("180");
  await page.getByRole("radio", { name: "已经用完", exact: true }).check();
  await expect(page.getByLabel("布片 1 长度", { exact: true })).toHaveCount(0);
  await page.getByRole("radio", { name: "还有剩余", exact: true }).check();
  await expect(page.getByLabel("布片 1 长度", { exact: true })).toHaveValue(
    "180",
  );
  await page.getByRole("button", { name: "添加另一组尺寸" }).click();
  await page.getByLabel("布片 2 幅宽", { exact: true }).fill("30");
  await page.getByLabel("布片 2 长度", { exact: true }).fill("50");
  await page.getByLabel("布片 2 不规则余料", { exact: true }).check();
  await page.getByLabel("布片 2 形状备注", { exact: true }).fill("缺角");
  await page.getByRole("button", { name: "保存余料", exact: true }).click();
  await expect(page.getByText("使用中", { exact: true })).toBeVisible();
  let result = await (
    await page.request.get(detail.replace("./fabrics/", "./api/fabrics/"))
  ).json();
  expect(result.pieces).toHaveLength(2);
  for (const field of ["name", "materials", "location", "price", "notes"])
    expect(result[field]).toEqual(original[field]);
  await page.getByRole("link", { name: "更新余料", exact: true }).click();
  await page.getByRole("radio", { name: "已经用完", exact: true }).check();
  await page.getByRole("button", { name: "标记已用完", exact: true }).click();
  await page.getByRole("button", { name: "继续修改", exact: true }).click();
  result = await (
    await page.request.get("./api/fabrics/" + original.id)
  ).json();
  expect(result.status).toBe("using");
  await page.getByRole("button", { name: "标记已用完", exact: true }).click();
  await page.getByRole("button", { name: "确认已用完", exact: true }).click();
  await expect(page.getByText("已用完", { exact: true })).toBeVisible();
  await expect(
    page.getByRole("link", { name: "更新余料", exact: true }),
  ).toHaveCount(0);
  result = await (
    await page.request.get("./api/fabrics/" + original.id)
  ).json();
  expect(result.pieces).toHaveLength(0);
  expect(result.notes).toBe(original.notes);
});
test("remnant drafts survive interrupted responses and stay separate from details", async ({
  page,
}) => {
  const created = await page.request.post("./api/fabrics", {
    data: {
      ...newFabric(),
      name: "余料恢复 " + Date.now(),
      pieces: [{ ...newPiece(), width: "150", length: "250" }],
    },
    headers: { "Idempotency-Key": randomBytes(16).toString("hex") },
  });
  expect(created.ok()).toBe(true);
  const fabric = await created.json();
  await page.goto("./fabrics/" + fabric.id + "/edit");
  await page.getByLabel("布料名称").fill("未保存的资料草稿");
  page.on("dialog", (dialog) => dialog.accept());
  await page.goto("./fabrics/" + fabric.id + "/remnant");
  await expect(page.getByText(fabric.name, { exact: true })).toBeVisible();
  await page.getByLabel("布片 1 长度", { exact: true }).fill("175");
  await page.reload();
  await expect(page.getByLabel("布片 1 长度", { exact: true })).toHaveValue(
    "175",
  );
  let release!: () => void, committed!: () => void;
  const released = new Promise<void>((resolve) => (release = resolve));
  const done = new Promise<void>((resolve) => (committed = resolve));
  await page.route("**/api/fabrics/" + fabric.id, async (route) => {
    if (route.request().method() !== "PUT") return route.continue();
    const response = await route.fetch();
    expect(response.ok()).toBe(true);
    expect(Object.keys(route.request().postDataJSON()).sort()).toEqual([
      "action",
      "pieces",
      "revision",
      "status",
    ]);
    committed();
    await released;
    await route.abort().catch(() => {});
  });
  await page.getByRole("button", { name: "保存余料", exact: true }).click();
  await done;
  await page.reload();
  release();
  await expect(page).toHaveURL(new RegExp("/fabrics/" + fabric.id + "$"));
  await expect(page.getByText("150 × 175 cm", { exact: true })).toBeVisible();
  await expect(
    page.getByRole("heading", { name: fabric.name, exact: true }),
  ).toBeVisible();
  const changes = await (
    await page.request.get("./api/fabrics/" + fabric.id + "/changes")
  ).json();
  expect(changes).toHaveLength(2);
  expect(changes[0].action).toBe("remnant");
});
test("reload during a committed write preserves its operation key", async ({
  page,
}) => {
  const name = "提交恢复 " + Date.now();
  let createdID = "";
  let committed!: () => void;
  let release!: () => void;
  const committedPromise = new Promise<void>(
    (resolve) => (committed = resolve),
  );
  const releasePromise = new Promise<void>((resolve) => (release = resolve));
  await page.route("**/api/fabrics", async (route) => {
    if (route.request().method() !== "POST") return route.continue();
    const response = await route.fetch();
    expect(response.ok()).toBe(true);
    createdID = (await response.json()).id;
    committed();
    await releasePromise;
    await route.abort().catch(() => {});
  });
  await page.goto("./new");
  await page.getByLabel("布料名称").fill(name);
  await page.getByRole("button", { name: "保存布料", exact: true }).click();
  await committedPromise;
  const cached = await page.evaluate(() =>
    JSON.parse(sessionStorage.getItem("fabricworld:draft:new")!),
  );
  expect(cached.uncertain).toBe(true);
  expect(cached.operationKey).toHaveLength(32);
  page.on("dialog", (dialog) => dialog.accept());
  await page.reload();
  release();
  await expect(
    page.getByText("已恢复本次浏览会话中的草稿。", { exact: false }),
  ).toBeVisible();
  await page.getByLabel("布料名称").fill(name + "修改");
  await page.getByRole("button", { name: "保存布料", exact: true }).click();
  await expect(
    page.getByText("上次提交结果尚未确认，请先重新查询，避免重复保存。", {
      exact: false,
    }),
  ).toBeVisible();
  await page.getByRole("button", { name: "查询上次结果" }).click();
  await expect(page).toHaveURL(new RegExp("/fabrics/" + createdID + "$"));
  await expect(page.getByRole("heading", { name, exact: true })).toBeVisible();
  const response = await page.request.get(
    "./api/fabrics?q=" + encodeURIComponent(name),
  );
  expect((await response.json()).total).toBe(1);
});
