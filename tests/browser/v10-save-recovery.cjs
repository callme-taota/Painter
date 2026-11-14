
// Adapted for V8+ custom comboboxes; persistence and recovery assertions remain intact.
const coverLabels={auto:'自动设计',upload:'上传图片',none:'不显示封面',editorial:'深邃渐变',geometric:'流彩渐变',minimal:'柔光渐变',ink:'墨蓝',moss:'苔绿',clay:'陶土',plum:'梅紫'};
async function chooseCover(page,label,value){await page.getByRole('combobox',{name:label,exact:true}).click();await page.getByRole('option',{name:coverLabels[value],exact:true}).click()}
async function selectedCover(page,label){const text=(await page.getByRole('combobox',{name:label,exact:true,includeHidden:true}).locator('span').first().textContent()).trim();return Object.keys(coverLabels).find(k=>coverLabels[k]===text)}
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
async function openArticleInfo(page) {
  const details = page.locator(".editor-properties");
  if (!(await details.evaluate((element) => element.open)))
    await details.locator("summary").click();
}
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
  const id =
    process.env.POST_ID ||
    JSON.parse(fs.readFileSync(out + "frontend-business-qa.json"))
      .created_post_id;
  const checks = [],
    errors = [];
  let p = await ctx.newPage();
  p.on("pageerror", (e) => errors.push(e.message));
  await p.goto(base + "/studio/posts/" + id);
  await p.getByRole("button", { name: "Markdown 源码", exact: true }).click();
  await p
    .getByRole("textbox", { name: "Markdown 正文", exact: true })
    .waitFor();
  const original = await p.evaluate(
    (id) => fetch("/api/v1/studio/posts/" + id).then((r) => r.json()),
    id,
  );
  const offlineSummary = "离线修改摘要与封面 " + Date.now();
  await ctx.setOffline(true);
  await openArticleInfo(p);
  await p.getByLabel("摘要", { exact: true }).fill(offlineSummary);
  for(const name of ["显示创建时间","显示最后编辑时间","显示发布时间"])await p.getByLabel(name,{exact:true}).uncheck();
  await chooseCover(p,"封面来源","auto");
  await chooseCover(p,"版式","minimal");
  await chooseCover(p,"配色","clay");
  await p.waitForTimeout(1500);
  await p.evaluate(() => scrollTo(0, 0));
  await p.screenshot({
    path: out + "editor-offline-state.png",
    fullPage: true,
  });
  console.log(JSON.stringify({offline_diagnostic:{state:(await p.locator(".save-state").innerText()).includes("离线"),summary:(await p.getByLabel("摘要",{exact:true}).inputValue())===offlineSummary,template:await selectedCover(p,"版式"),color:await selectedCover(p,"配色"),updated:await p.getByLabel("显示最后编辑时间",{exact:true}).isChecked()}}));
  checks.push({
    check: "offline status and retained summary/cover/date choices",
    passed:
      (await p.locator(".save-state").innerText()).includes("离线") &&
      (await p.getByLabel("摘要", { exact: true }).inputValue()) ===
        offlineSummary && (await selectedCover(p,"版式")) === "minimal" && (await selectedCover(p,"配色")) === "clay" && !await p.getByLabel("显示最后编辑时间",{exact:true}).isChecked(),
  });
  await p.close();
  await ctx.setOffline(false);
  p = await ctx.newPage();
  p.on("pageerror", (e) => errors.push(e.message));
  await p.goto(base + "/studio/posts/" + id);
  await p.getByRole("button", { name: "Markdown 源码", exact: true }).click();
  await p.getByText("发现尚未同步的本地内容", { exact: true }).waitFor();
  const unchanged = await p.evaluate(
    (id) => fetch("/api/v1/studio/posts/" + id).then((r) => r.json()),
    id,
  );
  checks.push({
    check:
      "summary/cover recovery after offline tab close leaves server unchanged",
    passed: unchanged.summary === original.summary && JSON.stringify(unchanged.cover) === JSON.stringify(original.cover),
  });
  await p.getByRole("button", { name: "加载恢复点", exact: true }).click();
  const recoveredSave = p.waitForResponse(
    (r) =>
      r.url().endsWith("/studio/posts/" + id + "/draft") &&
      r.request().method() === "PUT" &&
      r.ok(),
  );
  await p.getByRole("button", { name: "保存草稿", exact: true }).click();
  await recoveredSave;
  await p.locator(".save-state.saved").waitFor();
  checks.push({
    check: "recovered summary and cover persist to server",
    passed:
      (
        await p.evaluate(
          (id) => fetch("/api/v1/studio/posts/" + id).then((r) => r.json()),
          id,
        )
      ).summary === offlineSummary && await p.evaluate(async id=>{const d=await fetch("/api/v1/studio/posts/"+id).then(r=>r.json());return d.cover.template==="minimal"&&d.cover.color==="clay"&&!d.date_display.created&&!d.date_display.updated&&!d.date_display.published},id),
  });
  let entered, release;
  const enteredPromise = new Promise((r) => (entered = r)),
    releasePromise = new Promise((r) => (release = r));
  let first = true;
  await p.route("**/api/v1/studio/posts/" + id + "/draft", async (route) => {
    if (first) {
      first = false;
      entered();
      await releasePromise;
    }
    await route.continue();
  });
  const a = "慢响应前一稿 " + Date.now(),
    b = "慢响应期间继续输入 " + Date.now();
  await openArticleInfo(p);
  await p.getByLabel("摘要", { exact: true }).fill(a);
  await chooseCover(p,"版式","editorial");
  const firstDone = p.waitForResponse(
    (r) =>
      r.url().endsWith("/studio/posts/" + id + "/draft") &&
      r.request().method() === "PUT" &&
      r.ok(),
  );
  await p.getByRole("button", { name: "保存草稿", exact: true }).click();
  await enteredPromise;
  await openArticleInfo(p);
  await p.getByLabel("摘要", { exact: true }).fill(b);
  await chooseCover(p,"版式","geometric");
  await chooseCover(p,"配色","moss");
  release();
  await firstDone;
  checks.push({
    check: "old response does not overwrite new in-flight edits",
    passed: (await p.getByLabel("摘要", { exact: true }).inputValue()) === b && (await selectedCover(p,"版式")) === "geometric" && (await selectedCover(p,"配色")) === "moss",
  });
  await p.locator(".save-state.saved").waitFor();
  const after = await p.evaluate(
    (id) => fetch("/api/v1/studio/posts/" + id).then((r) => r.json()),
    id,
  );
  checks.push({
    check: "serial follow-up autosave captures newer snapshot",
    passed: after.summary === b && after.cover.template === "geometric" && after.cover.color === "moss",
  });
  await p.unroute("**/api/v1/studio/posts/" + id + "/draft");
  const cm = p.getByRole("textbox", { name: "Markdown 正文", exact: true });
  await cm.click();
  await p.keyboard.press("Control+a");
  await p.keyboard.insertText("![外链图片](https://example.com/blocked.png)");
  const invalid = p.waitForResponse(
    (r) =>
      r.url().endsWith("/studio/posts/" + id + "/draft") &&
      r.request().method() === "PUT",
  );
  await p.getByRole("button", { name: "保存草稿", exact: true }).click();
  const invalidResponse = await invalid;
  await p.getByText("保存失败 · 本地内容已保留", { exact: true }).waitFor();
  await p.evaluate(() => scrollTo(0, 0));
  await p.screenshot({
    path: out + "editor-validation-error.png",
    fullPage: true,
  });
  checks.push({
    check: "invalid external image rejected without losing local text",
    passed:
      invalidResponse.status() === 400 &&
      (await cm.innerText()).includes("blocked.png"),
  });
  await cm.click();
  await p.keyboard.press("Control+a");
  await p.keyboard.insertText(original.markdown);
  await openArticleInfo(p);
  await p.getByLabel("摘要", { exact: true }).fill(original.summary || "");
  for(const [key,name] of [["created","显示创建时间"],["updated","显示最后编辑时间"],["published","显示发布时间"]])await p.getByLabel(name,{exact:true}).setChecked(original.date_display[key]);
  await chooseCover(p,"封面来源",original.cover.mode);
  if(original.cover.mode === "auto") {
    await chooseCover(p,"版式",original.cover.template);
    await chooseCover(p,"配色",original.cover.color);
  }
  const corrected = p.waitForResponse(
    (r) =>
      r.url().endsWith("/studio/posts/" + id + "/draft") &&
      r.request().method() === "PUT" &&
      r.ok(),
  );
  await p.getByRole("button", { name: "保存草稿", exact: true }).click();
  await corrected;
  await p.locator(".save-state.saved").waitFor();
  checks.push({
    check:
      "correcting validation error saves fresh command instead of failed snapshot",
    passed: !(
      await p.evaluate(
        (id) => fetch("/api/v1/studio/posts/" + id).then((r) => r.json()),
        id,
      )
    ).markdown.includes("blocked.png"),
  });
  const d = p.waitForEvent("download");
  await p.getByRole("button", { name: "导出", exact: true }).click();
  const downloaded = await d;
  checks.push({
    check: "Markdown export download",
    passed: downloaded.suggestedFilename().endsWith(".md"),
  });
  await p.evaluate(() => scrollTo(0, 0));
  await p.screenshot({
    path: out + "desktop-save-recovery-final.png",
    fullPage: true,
  });
  fs.writeFileSync(
    out + "frontend-save-qa.json",
    JSON.stringify({ checks, errors, post_id: id }, null, 2),
  );
  console.log(JSON.stringify({ checks, errors, post_id: id }));
  await browser.close();
  if (checks.some((check) => check.passed === false) || errors.length)
    throw new Error("Browser acceptance failed; inspect the JSON artifact");
})().catch((e) => {
  console.error(e.stack);
  process.exit(1);
});
