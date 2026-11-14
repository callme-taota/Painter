
// Adapted for V8+ custom comboboxes; persistence and recovery assertions remain intact.
const coverLabels={auto:'自动设计',upload:'上传图片',none:'不显示封面',editorial:'深邃渐变',geometric:'流彩渐变',minimal:'柔光渐变',ink:'墨蓝',moss:'苔绿',clay:'陶土',plum:'梅紫'};
async function chooseCover(page,label,value){await page.getByRole('combobox',{name:label,exact:true}).click();await page.getByRole('option',{name:coverLabels[value],exact:true}).click()}
async function selectedCover(page,label){const text=(await page.getByRole('combobox',{name:label,exact:true,includeHidden:true}).locator('span').first().textContent()).trim();return Object.keys(coverLabels).find(k=>coverLabels[k]===text)}
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || "playwright");
const { randomUUID } = require("node:crypto");
const fs = require("fs"),
  path = require("path");
const base = process.env.BASE_URL || "http://127.0.0.1:18080",
  out =
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
  let p = await ctx.newPage();
  const checks = [],
    errors = [];
  p.on("pageerror", (e) => errors.push(e.message));
  await p.goto(base + "/studio/posts/new");
  await p.getByRole("button", { name: "Markdown 源码", exact: true }).click();
  await p
    .getByRole("textbox", { name: "Markdown 正文", exact: true })
    .waitFor();
  const me = await p.evaluate(() => fetch("/api/v1/me").then((r) => r.json()));
  const title = "离线新稿找回验收 " + Date.now(),
    summary = "仅保存在本机，关闭标签后仍可选择恢复。",
    markdown = "## 尚未联机的新稿\n\n中文正文、摘要和主题应一起保留。";
  await p.evaluate(async (client) => {
    await new Promise((resolve, reject) => {
      const req = indexedDB.open("painter-next-recovery", 1);
      req.onupgradeneeded = () => req.result.createObjectStore("drafts");
      req.onerror = () => reject(req.error);
      req.onsuccess = () => {
        const db = req.result,
          tx = db.transaction("drafts", "readwrite");
        tx.objectStore("drafts").put(
          {
            title: "其他账号不可见的新稿",
            summary: "foreign prefix fixture",
            markdown: "其他账号内容",
            client_draft_id: client,
            seq: 1,
            ack_seq: 0,
            saved_at: Date.now(),
          },
          `painter:${location.origin}:foreign-user-qa:${client}`,
        );
        tx.oncomplete = () => {
          db.close();
          resolve();
        };
        tx.onerror = () => reject(tx.error);
      };
    });
  }, randomUUID());
  const tagInfo = (await ctx.request.get(base + "/api/v1/tags?limit=50").then(r=>r.json())).items[0];
  if (!tagInfo) throw Error("The disposable recovery fixture needs one selectable tag");
  const tag = String(tagInfo.id), tagName = tagInfo.name;
  await ctx.setOffline(true);
  await p.getByRole("textbox", { name: "文章标题", exact: true }).fill(title);
  await openArticleInfo(p);
  await p.getByLabel("摘要", { exact: true }).fill(summary);
  await chooseCover(p,"版式","minimal");
  await chooseCover(p,"配色","plum");
  await p.getByLabel("显示最后编辑时间",{exact:true}).uncheck();
  await p.getByLabel("显示发布时间",{exact:true}).uncheck();
  await p.getByRole("combobox", { name: "标签", exact: true }).fill(tagName);
  await p.getByRole("option", { name: tagName, exact: true }).click();
  await p.getByRole("textbox", { name: "Markdown 正文", exact: true }).click();
  await p.keyboard.insertText(markdown);
  await p.waitForTimeout(1500);
  checks.push({
    check: "offline new article remains local before first create",
    passed:
      (await p.locator(".save-state").innerText()).includes("离线") &&
      p.url().endsWith("/new"),
  });
  const local = await p.evaluate(
    async (title) =>
      new Promise((resolve, reject) => {
        const req = indexedDB.open("painter-next-recovery", 1);
        req.onsuccess = () => {
          const db = req.result,
            tx = db.transaction("drafts"),
            q = tx.objectStore("drafts").openCursor();
          q.onsuccess = () => {
            const c = q.result;
            if (!c) return;
            if (c.value.title === title) {
              resolve({ key: String(c.key), ...c.value });
              return;
            }
            c.continue();
          };
          tx.oncomplete = () => db.close();
        };
        req.onerror = () => reject(req.error);
      }),
    title,
  );
  checks.push({check:"uncreated IndexedDB snapshot retains cover and selected tags",passed:local.cover?.mode==="auto"&&local.cover?.template==="minimal"&&local.cover?.color==="plum"&&local.tag_ids.includes(tag)&&local.date_display.created&&!local.date_display.updated&&!local.date_display.published});
  await p.close();
  await ctx.setOffline(false);
  p = await ctx.newPage();
  p.on("pageerror", (e) => errors.push(e.message));
  await p.goto(base + "/studio/posts/new");
  await p.getByRole("button", { name: "Markdown 源码", exact: true }).click();
  await p
    .getByRole("heading", { name: "找回尚未同步的新稿", exact: true })
    .waitFor();
  checks.push({
    check:
      "new tab discovers own uncreated draft and excludes foreign-user prefix",
    passed:
      (await p.getByText(title, { exact: true }).count()) === 1 &&
      (await p.getByText("其他账号不可见的新稿", { exact: true }).count()) ===
        0,
  });
  let row = p
    .locator(".user-row")
    .filter({ has: p.getByText(title, { exact: true }) });
  await row.getByRole("button", { name: "恢复这篇新稿", exact: true }).click();
  await openArticleInfo(p);
  checks.push({
    check:
      "explicit restore preserves title/summary/body/tags/cover/client identity",
    passed:
      (await p
        .getByRole("textbox", { name: "文章标题", exact: true })
        .inputValue()) === title &&
      (await p.getByLabel("摘要", { exact: true }).inputValue()) === summary &&
      (
        await p
          .getByRole("textbox", { name: "Markdown 正文", exact: true })
          .innerText()
      ).includes("尚未联机的新稿") &&
      (await selectedCover(p,"版式")) === "minimal" &&
      (await selectedCover(p,"配色")) === "plum" &&
      (await p.getByRole("button", { name: `移除标签：${tagName}`, exact: true }).isVisible()) && !await p.getByLabel("显示最后编辑时间",{exact:true}).isChecked() && !await p.getByLabel("显示发布时间",{exact:true}).isChecked(),
  });
  let createdId = "",
    createCount = 0;
  await p.route("**/api/v1/posts", async (route) => {
    if (route.request().method() !== "POST") return route.continue();
    createCount++;
    const response = await route.fetch();
    if (!response.ok())
      throw new Error("first creation failed: " + response.status());
    createdId = String((await response.json()).id);
    await route.abort("failed");
  });
  await p.getByRole("button", { name: "保存草稿", exact: true }).click();
  await p.getByText("保存失败 · 本地内容已保留", { exact: true }).waitFor();
  checks.push({
    check:
      "server accepted first create while lost response retained local draft",
    passed: createCount === 1 && !!createdId,
  });
  await p.close();
  p = await ctx.newPage();
  p.on("pageerror", (e) => errors.push(e.message));
  let repeatedCreate = 0,
    mappingReads = 0;
  p.on("request", (r) => {
    if (r.method() === "POST" && new URL(r.url()).pathname === "/api/v1/posts")
      repeatedCreate++;
    if (r.method() === "GET" && r.url().includes("/me/posts/by-client-id/"))
      mappingReads++;
  });
  await p.goto(base + "/studio/posts/new");
  await p.getByRole("button", { name: "Markdown 源码", exact: true }).click();
  await p
    .getByRole("heading", { name: "找回尚未同步的新稿", exact: true })
    .waitFor();
  row = p
    .locator(".user-row")
    .filter({ has: p.getByText(title, { exact: true }) });
  await row.getByRole("button", { name: "恢复这篇新稿", exact: true }).click();
  await p.getByRole("button", { name: "保存草稿", exact: true }).click();
  await p.waitForURL(base + "/studio/posts/" + createdId);
  await p.locator(".save-state.saved").waitFor();
  checks.push({
    check:
      "lost-response recovery queries original client ID and maps exactly one post",
    passed: mappingReads === 1 && repeatedCreate === 0,
  });
  const remote = await p.evaluate(
    (id) => fetch("/api/v1/studio/posts/" + id).then((r) => r.json()),
    createdId,
  );
  checks.push({
    check: "first server record contains complete recovered snapshot",
    passed:
      remote.title === title &&
      remote.summary === summary &&
      remote.markdown === markdown &&
      remote.cover.mode === "auto" && remote.cover.template === "minimal" && remote.cover.color === "plum" &&
      remote.tag_ids.includes(tag) && remote.date_display.created && !remote.date_display.updated && !remote.date_display.published,
  });
  const oldKey = local.key;
  await p.waitForFunction(
    async (key) =>
      new Promise((resolve) => {
        const req = indexedDB.open("painter-next-recovery", 1);
        req.onsuccess = () => {
          const db = req.result,
            q = db.transaction("drafts").objectStore("drafts").get(key);
          q.onsuccess = () => {
            resolve(!q.result);
            db.close();
          };
        };
      }),
    oldKey,
  );
  const mapping = await p.evaluate(
    async ({ user, id }) =>
      new Promise((resolve) => {
        const req = indexedDB.open("painter-next-recovery", 1);
        req.onsuccess = () => {
          const db = req.result,
            q = db
              .transaction("drafts")
              .objectStore("drafts")
              .get(`painter:${location.origin}:${user}:${id}`);
          q.onsuccess = () => {
            resolve(q.result);
            db.close();
          };
        };
      }),
    { user: me.id, id: createdId },
  );
  checks.push({
    check: "post-ID recovery is durable before uncreated record is removed",
    passed: mapping.post_id === createdId && mapping.title === title && mapping.cover?.template === "minimal" && mapping.cover?.color === "plum" && !mapping.date_display.updated && !mapping.date_display.published,
  });
  await p.evaluate(() => scrollTo(0, 0));
  await p.screenshot({
    path: out + "desktop-new-draft-recovered.png",
    fullPage: true,
  });
  await p.goto(base + "/studio/posts/new");
  await p.getByRole("button", { name: "Markdown 源码", exact: true }).click();
  await p
    .getByRole("textbox", { name: "Markdown 正文", exact: true })
    .waitFor();
  checks.push({
    check: "acknowledged new draft is not offered again",
    passed: (await p.getByText(title, { exact: true }).count()) === 0,
  });
  fs.writeFileSync(
    out + "frontend-new-draft-qa.json",
    JSON.stringify(
      {
        checks,
        errors,
        created_post_id: createdId,
        foreign_prefix_isolation: true,
      },
      null,
      2,
    ),
  );
  console.log(JSON.stringify({ checks, errors, created_post_id: createdId }));
  await browser.close();
  if (checks.some((c) => c.passed === false) || errors.length)
    throw new Error("Browser acceptance failed; inspect JSON artifact");
})().catch((e) => {
  console.error(e.stack);
  process.exit(1);
});
