import { test, expect, type Page } from "@playwright/test";
import { randomBytes } from "node:crypto";
import { newFabric } from "../src/types";

async function fits(page: Page) {
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  for (const popup of await page.locator('.dropdown-panel:popover-open:not([aria-hidden=true])').all()) {
    await expect(popup).toHaveCSS("opacity", "1");
    expect(await popup.evaluate(el => { const r = el.getBoundingClientRect(); return r.left >= 0 && r.right <= innerWidth + 1 && r.top >= 0 && r.bottom <= innerHeight + 1; })).toBe(true);
  }
}
async function choose(page: Page, label: string, value: string) {
  await page.getByRole("combobox", { name: label, exact: true }).click();
  await page.getByRole("option", { name: value, exact: true }).click();
}

test("library switch is in the top bar on every page and has one works entry", async ({ page }, info) => {
  for (const width of [320, 375, 1440]) {
    await page.setViewportSize({ width, height: width === 1440 ? 900 : 667 });
    await page.goto("./");
    const nav = page.getByRole("navigation", { name: "资料库", exact: true });
    await expect(nav.getByRole("link", { name: "布料库", exact: true })).toHaveAttribute("aria-current", "page");
    expect(await nav.evaluate(el => {
      const nav = el.getBoundingClientRect(), icon = document.querySelector('.brand-mark')!.getBoundingClientRect();
      return Math.abs((nav.top + nav.bottom) / 2 - (icon.top + icon.bottom) / 2) < 2;
    })).toBe(true);
    await nav.getByRole("link", { name: "成品集", exact: true }).click();
    await expect(page.getByRole("heading", { name: "我的成品", exact: true })).toBeVisible();
    await page.getByRole("button", { name: "打开菜单", exact: true }).click();
    await expect(page.getByRole("navigation", { name: "网站菜单", exact: true }).getByRole("link")).toHaveCount(2);
    await expect(page.getByRole("link", { name: "成品集", exact: true })).toHaveCount(1);
    await page.keyboard.press("Escape");
    await fits(page);
    await page.screenshot({ animations: "disabled", scale: "css", path: `../.local/custom-header-${info.project.name}-${width}.png` });
    await page.getByRole("link", { name: "记录成品", exact: true }).click();
    await expect(nav.getByRole("link", { name: "成品集", exact: true })).toHaveAttribute("aria-current", "page");
    await expect(page.locator("select,datalist")).toHaveCount(0);
    await fits(page);
  }
});

test("custom sorting and modal filters preserve keyboard focus and query values", async ({ page, request }, info) => {
  const mark = "下拉验收" + Date.now(), location = "收纳柜" + "很长的位置名称".repeat(16);
  const response = await request.post("./api/fabrics", { headers: { "Idempotency-Key": randomBytes(16).toString("hex") }, data: {
    ...newFabric(), name: mark, materials: [mark], location, color: mark, tags: [mark],
  } });
  expect(response.ok()).toBe(true);
  await page.setViewportSize({ width: 320, height: 667 });
  await page.goto("./");
  const sort = page.getByRole("combobox", { name: "排序", exact: true });
  await sort.focus(); await sort.press("ArrowDown"); await sort.press("End"); await sort.press("Enter");
  await expect(page).toHaveURL(/sort=name/);
  await expect(sort).toBeFocused();
  await sort.click(); await page.getByRole("heading", { name: "我的布料" }).click();
  await expect(page.getByRole("listbox")).toHaveCount(0);
  await page.locator(".filter-button").click();
  const modal = page.getByRole("dialog", { name: "筛选布料", exact: true });
  await modal.getByRole("combobox", { name: "材质", exact: true }).click();
  await page.keyboard.press("Escape");
  await expect(modal).toBeVisible();
  await expect(modal.getByRole("combobox", { name: "材质", exact: true })).toBeFocused();
  await choose(page, "材质", mark);
  await choose(page, "颜色", mark);
  await choose(page, "标签", mark);
  for (const width of [320, 375, 1440]) {
    await page.setViewportSize({ width, height: width === 1440 ? 900 : 667 });
    await modal.getByRole("combobox", { name: "收纳位置", exact: true }).click();
    await expect(page.getByRole("option", { name: location, exact: true })).toBeVisible();
    await fits(page);
    await page.screenshot({ animations: "disabled", scale: "css", path: `../.local/custom-filter-${info.project.name}-${width}.png` });
    await page.keyboard.press("Escape");
  }
  await choose(page, "收纳位置", location);
  await modal.getByRole("button", { name: "查看结果", exact: true }).click();
  await expect(page.locator(".fabric-card")).toHaveCount(1);
  await expect(page.locator(".fabric-card h2")).toHaveText(mark);
  await expect(page.locator("select,datalist")).toHaveCount(0);
  await page.emulateMedia({ reducedMotion: "reduce" });
  await sort.click();
  expect(await page.locator('.dropdown-panel:popover-open').evaluate(el => getComputedStyle(el).transitionDuration)).toBe("0s");
  await page.keyboard.press("Escape");
});

test("units convert dimensions and cancelling a used status preserves the values", async ({ page }) => {
  await page.goto("./new");
  await page.getByLabel("布片 1 幅宽", { exact: true }).fill("150");
  await page.getByLabel("布片 1 长度", { exact: true }).fill("230");
  await choose(page, "布片 1 单位", "m");
  await expect(page.getByLabel("布片 1 幅宽", { exact: true })).toHaveValue("1.5");
  await expect(page.getByLabel("布片 1 长度", { exact: true })).toHaveValue("2.3");
  await choose(page, "布片 1 单位", "cm");
  page.once("dialog", dialog => dialog.dismiss());
  await choose(page, "当前状态", "已用完");
  await expect(page.getByRole("combobox", { name: "当前状态", exact: true })).toHaveText("未使用");
  await expect(page.getByLabel("布片 1 幅宽", { exact: true })).toHaveValue("150");
  page.once("dialog", dialog => dialog.accept());
  await choose(page, "当前状态", "已用完");
  await expect(page.getByLabel("布片 1 幅宽", { exact: true })).toHaveCount(0);
  await choose(page, "当前状态", "未使用");
  await expect(page.getByLabel("布片 1 幅宽", { exact: true })).toHaveValue("");
});

test("editable suggestions allow custom text and keep a single open panel", async ({ page }) => {
  await page.goto("./works/new");
  const category = page.getByRole("combobox", { name: "类别", exact: true });
  await category.fill("上");
  await expect(page.getByRole("option", { name: "上衣", exact: true })).toBeVisible();
  await category.press("ArrowDown"); await category.press("Enter");
  await expect(category).toHaveValue("上衣");
  await expect(category).toBeFocused();
  await category.fill("自制小布偶");
  await expect(page.getByText("没有匹配的建议，可以直接填写。")).toBeVisible();
  await category.press("Enter");
  await expect(page).toHaveURL(/\/works\/new$/);
  await expect(category).toHaveValue("自制小布偶");
  await category.click();
  await category.press("Tab");
  await expect(page.getByRole("listbox")).toHaveCount(0);
  await expect(page.getByLabel(/完成日期/)).toBeFocused();
  await expect(page.getByLabel(/完成日期/)).toHaveAttribute("type", "date");
});
