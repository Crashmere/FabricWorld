import { test, expect, type Page } from "@playwright/test";

async function checkDateBounds(page: Page) {
  const bounds = await page.getByLabel("购买日期").evaluate((input) => {
    const date = input.getBoundingClientRect();
    const label = input.closest("label")!.getBoundingClientRect();
    const price = document
      .querySelector(".purchase-fields input[inputmode=decimal]")!
      .getBoundingClientRect();
    return {
      insideLabel: date.left >= label.left && date.right <= label.right + 1,
      insideScreen: date.left >= 0 && date.right <= innerWidth,
      separate: price.top >= date.bottom || price.left >= date.right,
      noOverflow: document.documentElement.scrollWidth <= innerWidth,
      tapHeight: label.height,
    };
  });
  expect(bounds.insideLabel).toBe(true);
  expect(bounds.insideScreen).toBe(true);
  expect(bounds.separate).toBe(true);
  expect(bounds.noOverflow).toBe(true);
  expect(bounds.tapHeight).toBeGreaterThanOrEqual(48);
}

test("purchase date stays in bounds when empty, filled, focused and edited", async ({
  page,
}) => {
  await page.goto("./new");
  await page.getByText("更多细节", { exact: true }).click();
  await expect(page.getByLabel("购买日期")).toHaveAttribute("type", "date");
  // macOS WebKit does not have iOS bug 301648. Reproduce its extra padding
  // in width calculations so this check also catches the old padded input.
  await page.addStyleTag({
    content: 'input[type="date"] { box-sizing: content-box !important; }',
  });
  for (const width of [320, 375, 390, 430, 768, 1440]) {
    await page.setViewportSize({ width, height: 900 });
    for (const value of ["", "2026-09-19"]) {
      await page.getByLabel("购买日期").fill(value);
      await page.getByLabel("购买日期").focus();
      await expect(page.getByLabel("购买日期")).toHaveValue(value);
      await checkDateBounds(page);
      await page.getByLabel("购买总价 / 元").fill("1234.56");
      await checkDateBounds(page);
    }
  }

  await page.setViewportSize({ width: 375, height: 667 });
  await page.getByLabel("布料名称").fill("日期兼容测试 " + Date.now());
  await page.getByRole("button", { name: "保存布料", exact: true }).click();
  await page.getByRole("link", { name: "编辑资料", exact: true }).click();
  await expect(page.getByLabel("购买日期")).toHaveValue("2026-09-19");
  await expect(page.getByLabel("购买总价 / 元")).toHaveValue("1234.56");
  await checkDateBounds(page);
  // Model a complete native-picker clear. Desktop WebKit can retain stale
  // segments after synthetic change events; reset its value after dispatch.
  await page.getByLabel("购买日期").evaluate((input: HTMLInputElement) => {
    input.value = "";
    input.dispatchEvent(new Event("input", { bubbles: true }));
    input.dispatchEvent(new Event("change", { bubbles: true }));
    input.value = "";
  });
  await page.getByLabel("购买总价 / 元").focus();
  await checkDateBounds(page);
  await page.getByRole("button", { name: "保存修改", exact: true }).click();
  await page.getByRole("link", { name: "编辑资料", exact: true }).click();
  await expect(page.getByLabel("购买日期")).toHaveValue("");
  await expect(page.getByLabel("购买总价 / 元")).toHaveValue("1234.56");
  await checkDateBounds(page);
});
