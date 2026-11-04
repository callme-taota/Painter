const { chromium } = require(process.env.PLAYWRIGHT_MODULE || "playwright");
const { randomUUID } = require("node:crypto");
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
  const b = await chromium.launch({
    executablePath: process.env.CHROMIUM_EXECUTABLE,
    headless: true,
    args: ["--no-sandbox", "--disable-dev-shm-usage"],
  });
  const c = await b.newContext({
    storageState: process.env.BROWSER_STATE_FILE,
    viewport: { width: 1440, height: 1000 },
  });
  const p = await c.newPage();
  const checks = [],
    errors = [];
  p.on("pageerror", (e) => errors.push(e.message));
  await p.goto(base + "/me");
  await p.getByRole("textbox", { name: "公开昵称", exact: true }).waitFor();
  const before = await p.evaluate(() =>
    fetch("/api/v1/me").then((r) => r.json()),
  );
  if (!/^[A-Za-z0-9]/.test(before.username))
    await p
      .getByRole("textbox", { name: "用户名", exact: true })
      .fill("browser_qa");
  await p
    .locator("input[type=file]")
    .setInputFiles(
      process.env.UPLOAD_FIXTURE_FILE ||
        path.join(__dirname, "fixtures/avatar.png"),
    );
  await p.getByText("头像已上传，保存资料后生效。", { exact: true }).waitFor();
  await p
    .getByRole("textbox", { name: "公开简介", exact: true })
    .fill((before.bio || "") + "\n浏览器资料保存验收。");
  await p.getByRole("button", { name: "保存资料", exact: true }).click();
  await p.getByText("个人资料已更新。", { exact: true }).waitFor();
  await p.goto(base + "/users/" + before.id);
  await p.locator(".author-profile img.avatar").waitFor();
  checks.push({
    check: "profile save + uploaded avatar rendered publicly",
    passed: await p
      .locator(".author-profile img.avatar")
      .evaluate((i) => i.complete && i.naturalWidth > 0),
  });
  await p.evaluate(async ({ original, key }) => {
    const s = await fetch("/api/v1/session").then((r) => r.json()),
      m = await fetch("/api/v1/me").then((r) => r.json());
    const r = await fetch("/api/v1/me", {
      method: "PATCH",
      headers: {
        "Content-Type": "application/json",
        "X-CSRF-Token": s.csrf_token,
        "If-Match": '"' + m.version + '"',
        "Idempotency-Key": key,
      },
      body: JSON.stringify({
        bio: original.bio,
        avatar_media_id: original.avatar_media_id,
      }),
    });
    if (!r.ok) throw new Error("profile restore failed");
  }, { original: before, key: randomUUID() });
  await p.goto(base + "/studio/settings");
  await p.getByRole("button", { name: "保存网站信息", exact: true }).waitFor();
  await p.getByRole("button", { name: "保存网站信息", exact: true }).click();
  await p.getByText("网站信息已保存。", { exact: true }).waitFor();
  checks.push({ check: "site-profile form persists", passed: true });
  const oldFlags = await p.evaluate(() =>
    fetch("/api/v1/site").then((r) => r.json()),
  );
  await p.getByLabel("允许评论", { exact: true }).uncheck();
  await p.getByLabel("允许收藏", { exact: true }).uncheck();
  await p.getByLabel("允许关注", { exact: true }).uncheck();
  await p.getByRole("button", { name: "保存功能设置", exact: true }).click();
  await p
    .getByText("功能与通知设置已保存。", { exact: true })
    .waitFor()
    .catch(async () => {
      await p
        .getByText("功能与通知设置已保存", { exact: false })
        .waitFor()
        .catch(async () => {
          await p
            .getByText("功能与通知设置已保存。", { exact: false })
            .waitFor();
        });
    });
  await p.goto(base + "/posts/1");
  await p.locator("[data-post-island]").scrollIntoViewIfNeeded();
  await p
    .getByText("站点目前关闭了新评论，已有讨论仍然保留。", { exact: true })
    .waitFor();
  checks.push({
    check: "comments disabled reflected while comments remain readable",
    passed: (await p.locator("#comment-body").count()) === 0,
  });
  checks.push({
    check: "new bookmark invitation hidden when disabled",
    passed:
      (await p.getByRole("button", { name: "收藏", exact: true }).count()) ===
      0,
  });
  await p.goto(base + "/users/1");
  await p.getByText("站点目前关闭了新关注。", { exact: true }).waitFor();
  checks.push({ check: "follow disabled reflected", passed: true });
  await p.goto(base + "/studio/settings");
  await p
    .getByLabel("允许评论", { exact: true })
    .setChecked(oldFlags.comments_enabled);
  await p
    .getByLabel("允许收藏", { exact: true })
    .setChecked(oldFlags.collections_enabled);
  await p
    .getByLabel("允许关注", { exact: true })
    .setChecked(oldFlags.follows_enabled);
  await p.getByRole("button", { name: "保存功能设置", exact: true }).click();
  await p.waitForTimeout(500);
  if (await p.locator("[role=alert]").count())
    throw new Error(await p.locator("[role=alert]").innerText());
  await p.goto(base + "/studio/taxonomy");
  const form = p.locator("form.panel").first();
  const name = "浏览器验收主题 " + Date.now();
  await form.getByRole("textbox", { name: "名称", exact: true }).fill(name);
  await form
    .getByRole("textbox", { name: "说明", exact: true })
    .fill("真实接口创建与归档验收。");
  await form.getByRole("button", { name: "创建主题", exact: true }).click();
  await p.getByText("主题已创建。", { exact: true }).waitFor();
  let found = false;
  for (const f of await p.locator("form.user-row").all()) {
    if (
      (await f
        .getByRole("textbox", { name: "名称", exact: true })
        .inputValue()) === name
    ) {
      await f.getByRole("checkbox", { name: "归档", exact: true }).check();
      await f.getByRole("button", { name: "保存", exact: true }).click();
      await p.getByText("主题已更新。", { exact: true }).waitFor();
      found = true;
      break;
    }
  }
  checks.push({ check: "taxonomy create/archive form", passed: found });
  await p.goto(base + "/studio/posts");
  await p.getByRole("button", { name: "全站文章审核", exact: true }).click();
  await p.getByText("把博客变成一间愿意停留的书房", { exact: true }).waitFor();
  checks.push({ check: "owner site-wide review scope", passed: true });
  await p.evaluate(() => scrollTo(0, 0));
  await p.screenshot({
    path: out + "desktop-owner-review.png",
    fullPage: true,
  });
  await p.goto(base + "/");
  await p
    .getByRole("combobox", { name: "外观主题", exact: true })
    .selectOption("dark");
  checks.push({
    check: "dark theme",
    passed: (await p.locator("html").getAttribute("data-theme")) === "dark",
  });
  await p
    .getByRole("combobox", { name: "外观主题", exact: true })
    .selectOption("light");
  checks.push({
    check: "light theme",
    passed: (await p.locator("html").getAttribute("data-theme")) === "light",
  });
  await p
    .getByRole("combobox", { name: "外观主题", exact: true })
    .selectOption("system");
  await p.emulateMedia({ colorScheme: "dark" });
  await p.waitForTimeout(150);
  checks.push({
    check: "system theme follows media",
    passed: (await p.locator("html").getAttribute("data-theme")) === "dark",
  });
  await p.evaluate(() => scrollTo(0, 0));
  await p.screenshot({ path: out + "dark-home-desktop.png", fullPage: true });
  await p.setViewportSize({ width: 390, height: 844 });
  await p.goto(base + "/posts/1");
  await p.evaluate(() => scrollTo(0, 0));
  await p.screenshot({ path: out + "dark-article-mobile.png", fullPage: true });
  const styles = await p.evaluate(() => ({
    body: getComputedStyle(document.body).fontSize,
    h1: getComputedStyle(document.querySelector("h1")).fontSize,
    h2: getComputedStyle(document.querySelector("h2")).fontSize,
    overflow: document.documentElement.scrollWidth > innerWidth,
  }));
  checks.push({
    check: "mobile dark article no overflow",
    passed: !styles.overflow,
  });
  await p.goto(base + "/studio/posts/new");
  await p.locator(".cm-content").waitFor();
  checks.push({
    check: "CM6 accessible name",
    passed:
      (await p
        .getByRole("textbox", { name: "Markdown 正文", exact: true })
        .count()) === 1,
  });
  await p.evaluate(() => scrollTo(0, 0));
  await p.screenshot({ path: out + "dark-editor-mobile.png", fullPage: true });
  fs.writeFileSync(
    out + "frontend-admin-qa.json",
    JSON.stringify({ checks, errors, styles }, null, 2),
  );
  console.log(JSON.stringify({ checks, errors, styles }));
  await b.close();
  if (checks.some((check) => check.passed === false) || errors.length)
    throw new Error("Browser acceptance failed; inspect the JSON artifact");
})().catch((e) => {
  console.error(e.stack);
  process.exit(1);
});
