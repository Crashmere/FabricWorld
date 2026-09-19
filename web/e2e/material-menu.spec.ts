import { test, expect } from "@playwright/test";
import { randomBytes } from "node:crypto";
import { newFabric } from "../src/types";

test("add unused material from empty search and reuse it in the editor", async ({ page }) => {
  const name = "自定义零使用材质" + Date.now();
  await page.goto("./materials");
  const search = page.getByRole("searchbox", { name: "搜索材质" });
  await search.fill("   ");
  await expect(page.getByRole("button", { name: /^添加“/ })).toHaveCount(0);
  await search.fill(name);
  const add = page.getByRole("button", { name: "添加“" + name + "”", exact: true });
  await expect(add).toBeVisible();
  for (const width of [320, 375, 1440]) {
    await page.setViewportSize({ width, height: width === 1440 ? 900 : 667 });
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    expect(await add.evaluate(button => button.scrollWidth <= button.clientWidth)).toBe(true);
    await page.screenshot({ path: "../.local/material-add-" + test.info().project.name + "-" + width + ".png" });
  }
  await page.setViewportSize({ width: 375, height: 667 });
  await page.route("**/api/materials", async route => {
    if (route.request().method() === "POST") { await route.fetch(); await route.abort(); }
    else await route.continue();
  });
  await page.route("**/api/operations/**", route => route.abort());
  await add.click();
  await expect(page.getByRole("button", { name: "重试添加", exact: true })).toBeVisible();
  await page.unroute("**/api/materials");
  await page.unroute("**/api/operations/**");
  await page.reload();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await search.fill(name);
  const row = page.locator(".material-row").filter({ has: page.getByRole("heading", { name, exact: true }) });
  await expect(row).toContainText("0 条布料");
  await expect(add).toHaveCount(0);
  await row.getByRole("button").click();
  await expect(page.getByRole("dialog")).toContainText("目前没有布料使用它");
  await page.getByRole("button", { name: "确认移除", exact: true }).click();
  await expect(add).toBeVisible();
  await add.click();
  await expect(row).toContainText("0 条布料");
  await page.goto("./new");
  await page.getByLabel("布料名称", { exact: true }).fill("独立材质使用测试");
  await page.getByRole("button", { name, exact: true }).click();
  await expect(page.locator(".material-single-percentage")).toHaveText("100%");
  await page.getByRole("button", { name: "棉", exact: true }).click();
  await expect(page.getByLabel(name + "百分比", { exact: true })).toHaveValue("");
  await expect(page.getByLabel("棉百分比", { exact: true })).toHaveValue("");
  await page.getByRole("button", { name: "保存布料", exact: true }).click();
  await expect(page.getByRole("link", { name: "编辑资料", exact: true })).toBeVisible();
  await page.goto("./materials");
  await search.fill(name);
  await expect(row).toContainText("1 条布料");
  await row.getByRole("button").click();
  await page.getByRole("button", { name: "确认移除", exact: true }).click();
  await expect(add).toBeVisible();
});

test("header menu and material removal preserve fabrics and recover interrupted responses", async ({ page, request }) => {
  const name = "待管理材质 " + Date.now();
  const other = "保留材质 " + Date.now();
  const create = async (materials: string[], status = "unused") => {
    const response = await request.post("./api/fabrics", {
      headers: { "Idempotency-Key": randomBytes(16).toString("hex") },
      data: { ...newFabric(), name: "菜单管理测试", materials, status, pieces: status === "used" ? [] : newFabric().pieces, composition: "保留成分说明", materialPercentages: { [name]: "70", [other]: "30" } },
    });
    expect(response.ok()).toBe(true);
    return response.json();
  };
  const mixed = await create([name, other]);
  const single = await create([name], "used");
  const trash = await create([name]);
  const deleted = await request.delete("./api/fabrics/" + trash.id, { headers: { "Idempotency-Key": randomBytes(16).toString("hex") }, data: { revision: trash.revision } });
  expect(deleted.ok()).toBe(true);
  const errors: string[] = [];
  page.on("pageerror", error => errors.push(error.message));
  await page.goto("./");
  const menuButton = page.getByRole("button", { name: "打开菜单", exact: true });
  const menu = page.getByRole("navigation", { name: "网站菜单", exact: true });
  await expect(page.locator(".site-header").getByRole("link", { name: "布料库", exact: true })).toHaveCount(0);
  for (const width of [320, 375, 1440]) {
    await page.setViewportSize({ width, height: width === 1440 ? 900 : 667 });
    await menuButton.click();
    await expect(menuButton).toHaveAttribute("aria-expanded", "true");
    await expect(menu.getByRole("link", { name: "回收站", exact: true })).toBeVisible();
    await expect(menu.getByRole("link", { name: "材质管理", exact: true })).toBeVisible();
    expect(await menu.evaluate(element => { const box = element.getBoundingClientRect(); return box.left >= 0 && box.right <= innerWidth; })).toBe(true);
    await page.screenshot({ path: "../.local/menu-" + test.info().project.name + "-" + width + ".png" });
    await page.keyboard.press("Escape");
    await expect(menu).toHaveCount(0);
    await expect(menuButton).toBeFocused();
    await menuButton.press("ArrowDown");
    await expect(menu.getByRole("link", { name: "回收站", exact: true })).toBeFocused();
    await page.getByRole("heading", { name: "我的布料" }).click();
    await expect(menu).toHaveCount(0);
  }
  await menuButton.click();
  await menu.getByRole("link", { name: "材质管理", exact: true }).click();
  await expect(page).toHaveURL(/\/materials$/);
  await expect(menu).toHaveCount(0);
  const row = page.locator(".material-row").filter({ has: page.getByRole("heading", { name, exact: true }) });
  await expect(row).toContainText("3 条布料");
  await expect(row).toContainText("1 条在回收站");
  for (const width of [320, 375, 1440]) {
    await page.setViewportSize({ width, height: width === 1440 ? 900 : 667 });
    await row.getByRole("button").scrollIntoViewIfNeeded();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.screenshot({ path: "../.local/material-management-" + test.info().project.name + "-" + width + ".png" });
  }
  await page.setViewportSize({ width: 375, height: 667 });
  await row.getByRole("button").click();
  const modal = page.getByRole("dialog", { name: "移除材质？", exact: true });
  await expect(modal).toContainText("3 条布料");
  await modal.getByRole("button", { name: "取消", exact: true }).click();
  await expect(row).toBeVisible();
  // Another edit after opening the list forces a refresh before the destructive action.
  const edited = await request.put("./api/fabrics/" + mixed.id, { headers: { "Idempotency-Key": randomBytes(16).toString("hex") }, data: { ...mixed, notes: "并发修改保留" } });
  expect(edited.ok()).toBe(true);
  await row.getByRole("button").click();
  await modal.getByRole("button", { name: "确认移除", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText("列表已刷新");
  await expect(row).toContainText("3 条布料");
  // The server commits, but the browser loses the response and operation lookup.
  await page.route("**/api/materials/remove", async route => { await route.fetch(); await route.abort(); });
  await page.route("**/api/operations/**", route => route.abort());
  await row.getByRole("button").click();
  await modal.getByRole("button", { name: "确认移除", exact: true }).click();
  await expect(modal.getByRole("button", { name: "重试移除", exact: true })).toBeVisible();
  await page.unroute("**/api/materials/remove");
  await page.unroute("**/api/operations/**");
  await page.reload();
  await expect(modal).toHaveCount(0);
  await expect(row).toHaveCount(0);
  const current = await (await request.get("./api/fabrics/" + mixed.id)).json();
  expect(current.materials).toEqual([other]);
  expect(current.materialPercentages).toEqual({ [other]: "100" });
  expect(current.notes).toBe("并发修改保留");
  expect(current.composition).toBe("保留成分说明");
  for (const original of [single, trash]) {
    const record = await (await request.get("./api/fabrics/" + original.id)).json();
    expect(record.materials).toEqual([]);
    expect(record.composition).toBe("保留成分说明");
    if (original.id === trash.id) expect(record.deletedAt).toBeTruthy();
  }
  await menuButton.click();
  await menu.getByRole("link", { name: "回收站", exact: true }).click();
  await expect(page.getByRole("heading", { name: "回收站", exact: true })).toBeVisible();
  await page.getByRole("link", { name: "FabricWorld 布料库", exact: true }).click();
  await expect(page.getByRole("heading", { name: "我的布料" })).toBeVisible();
  expect(errors).toEqual([]);
});
