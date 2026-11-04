const { chromium } = require(process.env.PLAYWRIGHT_MODULE || "playwright");
const fs = require("fs");
const path = require("path");
const base = process.env.BASE_URL || "http://127.0.0.1:18080";
const out =
  path.resolve(
    process.env.ARTIFACT_DIR || path.join(__dirname, "../artifacts"),
  ) + path.sep;
fs.mkdirSync(out, { recursive: true });
if (!process.env.BROWSER_STATE_FILE)
  throw new Error(
    "BROWSER_STATE_FILE is required; use a disposable QA database",
  );
(async () => {
  const browser = await chromium.launch({
    executablePath: process.env.CHROMIUM_EXECUTABLE,
    headless: true,
    args: ["--no-sandbox", "--disable-dev-shm-usage"],
  });
  const ctx = await browser.newContext({
    storageState: process.env.BROWSER_STATE_FILE,
    viewport: { width: 1440, height: 1000 },
  });
  const page = await ctx.newPage();
  const errors = [],
    requests = [];
  page.on("pageerror", (e) => errors.push(e.message));
  page.on("response", (r) => {
    if (r.url().includes("/api/v1/") && r.status() >= 400)
      requests.push({ url: new URL(r.url()).pathname, status: r.status() });
  });
  const checks = [];
  const title = "浏览器验收 · 中文写作与发布 " + Date.now();
  await page.goto(base + "/studio/posts/new");
  await page.locator(".cm-content").waitFor();
  await page
    .getByRole("textbox", { name: "文章标题", exact: true })
    .fill(title);
  await page.locator(".cm-content").click();
  await page.keyboard.insertText(
    '## 一个想法的开始\n\n这是一篇真实通过浏览器创建的验收文章。中文输入、草稿保存、发布与恢复需要彼此配合。\n\n```go\nfunc main() { println("你好，Painter") }\n```\n\n| 阶段 | 状态 |\n| --- | --- |\n| 写作 | 已准备 |',
  );
  await page.waitForURL(/\/studio\/posts\/\d+$/);
  const id = page.url().split("/").pop();
  await page.locator(".save-state.saved").waitFor();
  checks.push({ check: "create/autosave", passed: true, id });
  await page
    .locator("#editor-upload")
    .setInputFiles(
      process.env.UPLOAD_FIXTURE_FILE ||
        path.join(__dirname, "fixtures/avatar.png"),
    );
  await page.waitForFunction(() =>
    document.querySelector(".cm-content").textContent.includes("/media/"),
  );
  await page.locator(".save-state.saved").waitFor();
  checks.push({
    check: "upload placeholder replaced with ready local media",
    passed: true,
  });
  await page.evaluate(() => scrollTo(0, 0));
  await page.waitForTimeout(100);
  await page.screenshot({ path: out + "desktop-editor.png", fullPage: true });
  await page.getByRole("button", { name: "留检查点", exact: true }).click();
  await page.getByText("已创建不可变检查点。", { exact: true }).waitFor();
  checks.push({ check: "checkpoint", passed: true });
  await page.getByRole("button", { name: "发布文章", exact: true }).click();
  await page
    .getByText("文章已发布。后续草稿修改不会改变当前公开版本。", {
      exact: true,
    })
    .waitFor();
  checks.push({ check: "publish", passed: true });
  await page.goto(base + "/posts/" + id);
  await page.getByRole("heading", { name: title, exact: true }).waitFor();
  if (!(await page.locator(".prose").innerText()).includes("一个想法的开始"))
    throw new Error("published body absent");
  await page.locator(".prose img").waitFor();
  checks.push({
    check: "uploaded image survives save/checkpoint/publish and public read",
    passed: await page
      .locator(".prose img")
      .evaluate((i) => i.complete && i.naturalWidth > 0),
  });
  await page.screenshot({
    path: out + "desktop-published-qa.png",
    fullPage: true,
  });
  await page.locator("[data-post-island]").scrollIntoViewIfNeeded();
  await page.locator("#comment-body").waitFor();
  await page.locator("#comment-body").fill("这条评论来自真实浏览器接口验收。");
  await page.getByRole("button", { name: "发表评论", exact: true }).click();
  await page
    .getByText("这条评论来自真实浏览器接口验收。", { exact: true })
    .waitFor();
  await page.getByRole("button", { name: /^点赞/ }).click();
  await page.getByRole("button", { name: "收藏", exact: true }).click();
  checks.push({ check: "comment/like/bookmark", passed: true });
  const tabA = await ctx.newPage(),
    tabB = await ctx.newPage();
  await tabA.goto(base + "/studio/posts/" + id);
  await tabB.goto(base + "/studio/posts/" + id);
  await tabA.locator(".cm-content").waitFor();
  await tabB.locator(".cm-content").waitFor();
  await tabA
    .getByRole("textbox", { name: "文章标题", exact: true })
    .fill(title + " A");
  await tabA.getByRole("button", { name: "保存草稿", exact: true }).click();
  await tabA.locator(".save-state.saved").waitFor();
  await tabB
    .getByRole("textbox", { name: "文章标题", exact: true })
    .fill(title + " B");
  await tabB.getByRole("button", { name: "保存草稿", exact: true }).click();
  await tabB.getByRole("heading", { name: "另一处修改已先保存" }).waitFor();
  await tabB.evaluate(() => scrollTo(0, 0));
  await tabB.waitForTimeout(100);
  await tabB.screenshot({ path: out + "desktop-conflict.png", fullPage: true });
  checks.push({
    check: "two-tab conflict retained local",
    passed: (
      await tabB
        .getByRole("textbox", { name: "文章标题", exact: true })
        .inputValue()
    ).endsWith(" B"),
  });
  await tabB
    .getByRole("button", { name: "以服务器最新版本为基准，继续合并" })
    .click();
  await tabB.getByRole("button", { name: "保存草稿", exact: true }).click();
  await tabB.locator(".save-state.saved").waitFor();
  checks.push({ check: "explicit conflict resolution", passed: true });
  await tabB.getByRole("button", { name: "查看修订记录", exact: true }).click();
  await tabB
    .getByRole("button", { name: "恢复为草稿", exact: true })
    .first()
    .waitFor();
  tabB.once("dialog", (d) => d.accept());
  await tabB
    .getByRole("button", { name: "恢复为草稿", exact: true })
    .first()
    .click();
  await tabB
    .getByText("历史版本已恢复为草稿，尚未发布。", { exact: true })
    .waitFor();
  checks.push({ check: "revision restore", passed: true });
  await tabB.getByRole("button", { name: "撤回", exact: true }).click();
  await tabB
    .getByText("文章已撤回，草稿和历史修订保留。", { exact: true })
    .waitFor();
  const response = await page.goto(base + "/posts/" + id);
  checks.push({
    check: "withdraw public 404",
    passed: response.status() === 404,
  });
  await page.goto(base + "/studio/posts");
  await page.getByText(title, { exact: true }).first().waitFor();
  const row = page
    .locator("tr")
    .filter({ has: page.getByText(title, { exact: true }) });
  await row.getByRole("button", { name: "移入回收站" }).click();
  await page.getByRole("button", { name: "回收站", exact: true }).click();
  const trashed = page
    .locator("tr")
    .filter({ has: page.getByText(title, { exact: true }) });
  await trashed.getByRole("button", { name: "恢复", exact: true }).click();
  checks.push({ check: "trash/recover", passed: true });
  for (const path of [
    "/me",
    "/me/sessions",
    "/me/media",
    "/me/bookmarks",
    "/me/following",
    "/me/history",
    "/studio/settings",
    "/studio/taxonomy",
    "/studio/members",
    "/studio/comments",
    "/studio/analytics",
    "/studio/operations",
  ]) {
    await page.goto(base + path);
    await page.locator(".work-heading").waitFor();
    await page.waitForTimeout(300);
    checks.push({
      check: "load " + path,
      passed: (await page.locator("[role=alert]").count()) === 0,
    });
  }
  fs.writeFileSync(
    out + "frontend-business-qa.json",
    JSON.stringify({ checks, errors, requests, created_post_id: id }, null, 2),
  );
  console.log(
    JSON.stringify({ checks, errors, requests, created_post_id: id }),
  );
  await browser.close();
  if (checks.some((check) => check.passed === false) || errors.length)
    throw new Error("Browser acceptance failed; inspect the JSON artifact");
})().catch((e) => {
  console.error(e.stack);
  process.exit(1);
});
