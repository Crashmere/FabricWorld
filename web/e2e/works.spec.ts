import { test, expect, type Page, type APIRequestContext } from "@playwright/test";
import { randomBytes } from "node:crypto";
import { newFabric } from "../src/types";
import { newWork } from "../src/works";

const headers = () => ({ "Idempotency-Key": randomBytes(16).toString("hex") });
async function create(request: APIRequestContext, kind: "works" | "fabrics", data: unknown) {
  const response = await request.post("./api/" + kind, { data, headers: headers() });
  expect(response.ok()).toBe(true);
  return response.json();
}
async function image(page: Page, color: string) {
  const data = await page.evaluate(color => {
    const c = document.createElement("canvas"); c.width = 640; c.height = 800;
    const x = c.getContext("2d")!; x.fillStyle = "#f0eee5"; x.fillRect(0, 0, 640, 800);
    x.fillStyle = color; x.beginPath();
    x.moveTo(220, 120); x.lineTo(130, 175); x.lineTo(80, 340); x.lineTo(165, 370);
    x.lineTo(205, 280); x.lineTo(190, 675); x.lineTo(450, 675); x.lineTo(435, 280);
    x.lineTo(475, 370); x.lineTo(560, 340); x.lineTo(510, 175); x.lineTo(420, 120);
    x.lineTo(365, 150); x.lineTo(275, 150); x.closePath(); x.fill();
    x.strokeStyle = "#f6f3e9"; x.lineWidth = 3; x.setLineDash([5, 5]); x.beginPath(); x.moveTo(320, 160); x.lineTo(320, 650); x.stroke();
    x.setLineDash([]); for (let y = 240; y < 600; y += 65) { x.beginPath(); x.arc(334, y, 4, 0, Math.PI * 2); x.stroke(); }
    return c.toDataURL("image/jpeg").split(",")[1]!;
  }, color);
  return { name: "synthetic-shirt.jpg", mimeType: "image/jpeg", buffer: Buffer.from(data, "base64") };
}
async function fits(page: Page) {
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
}

test("record photos, fabric links, details, search, export and recover a finished piece", async ({ page }, info) => {
  const name = "合成示例 · 秋日亚麻衬衫 " + Date.now();
  const fabric = await create(page.request, "fabrics", { ...newFabric(), name: "合成亚麻 " + Date.now(), materials: ["麻"] });
  await page.setViewportSize({ width: 375, height: 667 });
  await page.goto("./fabrics/" + fabric.id);
  await page.getByRole("link", { name: "记录成品", exact: true }).click();
  await expect(page.getByText(fabric.name, { exact: true })).toBeVisible();
  await page.getByLabel("成品名称", { exact: true }).fill(name);
  await page.getByLabel("类别", { exact: false }).fill("上衣");
  await page.getByLabel("完成日期").fill("2026-09-27");
  await page.getByLabel("纸样 / 教程").fill("基础衬衫 · 07 号纸样");
  await page.getByLabel("尺码", { exact: true }).fill("M，袖长缩短 2 cm");
  await page.getByLabel("为谁制作").fill("自己");
  await page.getByLabel("标签", { exact: true }).fill("春秋，日常");
  await page.getByLabel(fabric.name + " 用布说明").fill("主布约 1.5 米");
  await page.getByLabel("制作心得", { exact: true }).fill("第一次做翻领。下次把袖口收窄一些。\n保留了布边做挂袢。");
  await page.getByLabel("选择照片文件").setInputFiles([await image(page, "#859a86"), await image(page, "#bb9981")]);
  await expect(page.getByText("2 / 10", { exact: true })).toBeVisible({ timeout: 30000 });
  await page.getByRole("button", { name: "前移照片 2", exact: true }).click();
  await page.getByRole("button", { name: "查看成品照片 1 大图" }).click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await page.keyboard.press("Escape");
  for (const width of [320, 375, 1440]) {
    await page.setViewportSize({ width, height: width === 1440 ? 900 : 667 }); await fits(page);
    await page.screenshot({ path: `../.local/works-editor-${info.project.name}-${width}.png`, fullPage: true });
  }
  await page.getByRole("button", { name: "保存成品", exact: true }).click();
  await expect(page).toHaveURL(/\/works\/[a-f0-9]{32}$/);
  const detail = page.url(), id = detail.split("/").pop()!;
  await expect(page.getByRole("heading", { name, exact: true })).toBeVisible();
  await expect(page.getByText("主布约 1.5 米", { exact: true })).toBeVisible();
  const unchanged = await (await page.request.get("./api/fabrics/" + fabric.id)).json();
  expect(unchanged).toEqual(fabric);
  for (const width of [320, 375, 1440]) {
    await page.setViewportSize({ width, height: width === 1440 ? 900 : 667 }); await fits(page);
    await page.screenshot({ path: `../.local/works-detail-${info.project.name}-${width}.png`, fullPage: true });
  }
  await page.getByRole("link", { name: fabric.name, exact: true }).click();
  await expect(page.getByRole("link", { name: new RegExp(name) })).toBeVisible();
  await page.goto(detail + "/edit");
  await page.getByLabel("制作心得", { exact: true }).fill("编辑后的制作心得");
  await page.getByRole("button", { name: "保存修改", exact: true }).click();
  await page.getByRole("button", { name: "查看修改历史" }).click();
  await expect(page.getByText("首次记录", { exact: true })).toBeVisible();
  await page.goto("./works");
  await page.getByLabel("搜索成品", { exact: true }).fill(name);
  await page.getByRole("button", { name: "搜索", exact: true }).click();
  await expect(page.locator(".work-card")).toHaveCount(1);
  await page.getByLabel("成品类别筛选").selectOption("上衣");
  for (const width of [320, 375, 1440]) {
    await page.setViewportSize({ width, height: width === 1440 ? 900 : 667 }); await fits(page);
    await page.screenshot({ path: `../.local/works-library-${info.project.name}-${width}.png`, fullPage: true });
  }
  const download = page.waitForEvent("download");
  await page.getByRole("link", { name: "导出含照片", exact: true }).click();
  expect((await download).suggestedFilename()).toBe("fabricworld-works.zip");
  await page.goto(detail);
  await page.getByRole("button", { name: "删除成品", exact: true }).click();
  await page.getByRole("button", { name: "移入回收站", exact: true }).click();
  await expect(page).toHaveURL(/\/works$/);
  await page.getByRole("link", { name: "成品回收站", exact: true }).click();
  await page.locator(".work-card").filter({ hasText: name }).getByRole("link").click();
  await page.getByRole("button", { name: "恢复成品", exact: true }).click();
  await expect(page.getByRole("link", { name: "编辑成品", exact: true })).toBeVisible();
  expect((await (await page.request.get("./api/works/" + id)).json()).photos).toHaveLength(2);
});

test("find multiple fabrics, retain draft, and handle concurrent edits", async ({ page }) => {
  const mark = "用布查找" + Date.now();
  const a = await create(page.request, "fabrics", { ...newFabric(), name: mark + "主布" });
  const b = await create(page.request, "fabrics", { ...newFabric(), name: mark + "里布", status: "used", pieces: [] });
  await page.goto("./works/new");
  await page.getByLabel("成品名称", { exact: true }).fill("测试包袋 " + mark);
  await page.getByLabel("查找布料", { exact: true }).fill(mark);
  await page.getByRole("button", { name: "查找", exact: true }).click();
  await page.getByRole("button", { name: a.name, exact: true }).click();
  await page.getByRole("button", { name: b.name + " · 已用完", exact: true }).click();
  await page.getByLabel(a.name + " 用布说明").fill("外层");
  page.on("dialog", dialog => dialog.accept());
  await page.reload();
  await expect(page.getByLabel(a.name + " 用布说明")).toHaveValue("外层");
  await page.getByRole("button", { name: "保存成品", exact: true }).click();
  await expect(page).toHaveURL(/\/works\/[a-f0-9]{32}$/);
  const detail = page.url(), id = detail.split("/").pop()!;
  await page.goto(detail + "/edit");
  await page.getByLabel("制作心得", { exact: true }).fill("本页保留的心得");
  const current = await (await page.request.get("./api/works/" + id)).json();
  const changed = await page.request.put("./api/works/" + id, { data: { ...current, notes: "另一处已保存的心得" }, headers: headers() });
  expect(changed.ok()).toBe(true);
  await page.getByRole("button", { name: "保存修改", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText("另一处修改");
  await expect(page.getByLabel("制作心得", { exact: true })).toHaveValue("本页保留的心得");
  await page.reload();
  await expect(page.getByText("另一处已保存的心得", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "已核对，保留当前输入" }).click();
  await page.getByRole("button", { name: "保存修改", exact: true }).click();
  await expect(page.getByText("本页保留的心得", { exact: true })).toBeVisible();
  const result = await (await page.request.get("./api/works/" + id)).json();
  expect(result.revision).toBe(3); expect(result.fabrics).toHaveLength(2);
});

test("reload resolves committed creation and edit before comparing draft revisions", async ({ page }) => {
  for (const editing of [false, true]) {
    const created = editing ? await create(page.request, "works", { ...newWork(), name: "已存在的作品" }) : null;
    const path = editing ? "./works/" + created.id + "/edit" : "./works/new";
    let committed!: () => void, release!: () => void;
    const done = new Promise<void>(r => committed = r), released = new Promise<void>(r => release = r);
    const endpoint = editing ? "**/api/works/" + created.id : "**/api/works";
    await page.route(endpoint, async route => {
      if (route.request().method() !== (editing ? "PUT" : "POST")) return route.continue();
      const response = await route.fetch(); expect(response.ok()).toBe(true);
      committed(); await released; await route.abort().catch(() => {});
    });
    await page.goto(path);
    const name = "响应恢复" + editing + Date.now();
    await page.getByLabel("成品名称", { exact: true }).fill(name);
    await page.getByRole("button", { name: editing ? "保存修改" : "保存成品", exact: true }).click();
    await done;
    page.once("dialog", dialog => dialog.accept());
    await page.reload(); release();
    await expect(page).toHaveURL(/\/works\/[a-f0-9]{32}$/);
    await expect(page.getByRole("heading", { name, exact: true })).toBeVisible();
    const list = await (await page.request.get("./api/works?q=" + encodeURIComponent(name))).json();
    expect(list.total).toBe(1); expect(list.items[0].revision).toBe(editing ? 2 : 1);
    await page.unroute(endpoint);
  }
});

test("phone layouts, empty state, long text and camera controls", async ({ page }) => {
  for (const width of [320, 375, 1440]) {
    await page.setViewportSize({ width, height: width === 1440 ? 900 : 667 });
    await page.goto("./works/new");
    await expect(page.getByLabel("拍照上传")).toHaveAttribute("capture", "environment");
    await expect(page.getByLabel("选择照片文件")).toHaveAttribute("multiple", "");
    await fits(page);
  }
  const w = await create(page.request, "works", { ...newWork(), name: "LongName".repeat(24), tags: ["长标签".repeat(13)], notes: "LongWord".repeat(200), pattern: "LongPattern".repeat(25) });
  for (const width of [320, 375, 1440]) {
    await page.setViewportSize({ width, height: width === 1440 ? 900 : 667 });
    await page.goto("./works/" + w.id); await expect(page.getByRole("heading", { level: 1 })).toBeVisible(); await fits(page);
  }
  await page.goto("./works?q=no-match-" + Date.now());
  await expect(page.getByRole("heading", { name: "没有找到符合条件的成品" })).toBeVisible(); await fits(page);
});

test("failed work photos remain previewable and retry without changing their key", async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 667 });
  await page.goto("./works/new");
  await page.getByLabel("成品名称", { exact: true }).fill("上传重试 " + Date.now());
  const keys: string[] = [];
  await page.route("**/api/uploads", route => {
    keys.push(route.request().headers()["idempotency-key"]!);
    return route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ code: "busy", message: "合成上传失败" }) });
  });
  await page.getByLabel("选择照片文件").setInputFiles(await image(page, "#9aafbb"));
  await expect(page.getByRole("button", { name: "重试", exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "保存成品", exact: true })).toBeDisabled();
  await page.getByRole("button", { name: "查看待上传照片 synthetic-shirt.jpg", exact: true }).click({ position: { x: 12, y: 12 } });
  await expect(page.getByRole("dialog").locator("img")).toHaveAttribute("src", /^blob:/);
  await page.keyboard.press("Escape");
  await page.unroute("**/api/uploads");
  await page.route("**/api/uploads", route => { keys.push(route.request().headers()["idempotency-key"]!); return route.continue(); });
  await page.getByRole("button", { name: "重试", exact: true }).click();
  await expect(page.getByText("1 / 10", { exact: true })).toBeVisible();
  expect(keys).toHaveLength(2); expect(keys[0]).toBe(keys[1]);
  await expect(page.getByRole("button", { name: "保存成品", exact: true })).toBeEnabled();
  await page.getByRole("button", { name: "移除照片 1", exact: true }).click();
  await page.getByRole("button", { name: "保存成品", exact: true }).click();
  await expect(page.getByText("还没有成品照片", { exact: true })).toBeVisible();
});

test("unknown uncommitted save can retry the original request after reload", async ({ page }) => {
  const keys: string[] = [];
  await page.route("**/api/works", route => {
    if (route.request().method() !== "POST") return route.continue();
    keys.push(route.request().headers()["idempotency-key"]!);
    return route.abort();
  });
  await page.goto("./works/new");
  const name = "未提交恢复 " + Date.now();
  await page.getByLabel("成品名称", { exact: true }).fill(name);
  await page.getByRole("button", { name: "保存成品", exact: true }).click();
  await expect(page.getByRole("button", { name: "按原内容重试" })).toBeEnabled();
  page.on("dialog", dialog => dialog.accept());
  await page.reload();
  await expect(page.getByRole("button", { name: "按原内容重试" })).toBeEnabled();
  await page.unroute("**/api/works");
  await page.route("**/api/works", route => {
    if (route.request().method() === "POST") keys.push(route.request().headers()["idempotency-key"]!);
    return route.continue();
  });
  await page.getByRole("button", { name: "按原内容重试" }).click();
  await expect(page.getByRole("heading", { name, exact: true })).toBeVisible();
  expect(keys).toHaveLength(2); expect(keys[0]).toBe(keys[1]);
});

test("delete and restore recover committed operations after reload", async ({ page }) => {
  const w = await create(page.request, "works", { ...newWork(), name: "删除恢复 " + Date.now() });
  for (const action of ["delete", "restore"]) {
    const endpoint = "**/api/works/" + w.id + (action === "restore" ? "/restore" : "");
    let committed!: () => void, release!: () => void;
    const done = new Promise<void>(r => committed = r), released = new Promise<void>(r => release = r);
    await page.route(endpoint, async route => {
      if (route.request().method() !== (action === "delete" ? "DELETE" : "POST")) return route.continue();
      const response = await route.fetch(); expect(response.ok()).toBe(true);
      committed(); await released; await route.abort().catch(() => {});
    });
    await page.goto("./works/" + w.id);
    if (action === "delete") {
      await page.getByRole("button", { name: "删除成品", exact: true }).click();
      await page.getByRole("button", { name: "移入回收站", exact: true }).click();
    } else await page.getByRole("button", { name: "恢复成品", exact: true }).click();
    await done; await page.reload(); release();
    if (action === "delete") await expect(page).toHaveURL(/\/works$/);
    else await expect(page.getByRole("link", { name: "编辑成品", exact: true })).toBeVisible();
    await page.unroute(endpoint);
  }
  const result = await (await page.request.get("./api/works/" + w.id)).json();
  expect(result.revision).toBe(3); expect(result.deletedAt).toBeNull();
});
