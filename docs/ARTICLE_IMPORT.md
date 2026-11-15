# 结构化文章导入

Painter 可把 AI 整理结果或其他工具导出的 JSON 编译成普通文章 Markdown。导入先校验并预览，确认后写入当前编辑草稿；保存、冲突处理、检查点和发布继续使用原有流程。

配套文件：

- [完整 JSON Schema](painter-article.schema.json)：JSON Schema Draft 2020-12，所有对象拒绝未知字段。
- [可导入示例](examples/painter-article.json)：完全虚构的阅读记录，展示 Markdown、提示、指标和摘要卡片。
- [可直接复制的 AI 提示词](prompts/painter-article-export.md)：用于把原材料整理成此格式。
- [组合卡片与渲染管线](CONTENT_RENDER_PIPELINE.md)：组件、模板、兼容策略和扩展方式。

## 在编辑器中使用

1. 打开文章编辑器，在“更多”中选择“导入文章 JSON”。
2. 如需借助 AI 整理材料，可在面板中复制 AI 提示词；也可下载示例 JSON 或 Schema，供其他工具按相同格式生成文件。
3. 粘贴 JSON，或选择一个 UTF-8 `.json` 文件；提交前确认其中没有密码、会话、私有运行配置或其他秘密。
4. 点击“预览文章”，查看服务器生成的预览。发现格式、来源、图片或内容错误时，先修改输入，再次校验。
5. 选择“追加”或“替换”。默认追加至当前正文；替换会替换当前编辑正文，应先核对预览和现有内容。
6. 如需同时采用导入的标题与摘要，勾选“使用文件中的标题与摘要”；默认不导入这两项。
7. 确认应用后，改动进入正常草稿自动保存与版本冲突检查。需要公开时，仍须使用文章原有发布操作。

导入不会自动发布，也不更改标签、封面、日期设置或作者。预览本身不保存文章；应用导入内容后才产生普通草稿编辑。网络失败或版本冲突时，按编辑器已有提示保留、重查并解决当前修改。

## 文件结构

最小示例：

```json
{
  "format": "painter.article",
  "version": 1,
  "title": "我的文章",
  "blocks": [
    {"type": "markdown", "markdown": "## 正文\n\n这里是原材料明确提供的内容。"}
  ]
}
```

根对象只允许以下字段，不能增加 `$schema`、`author`、`tags`、`cover`、`html` 或发布指令：

| 字段 | 规则 |
| --- | --- |
| `format` | 必填，精确为 `"painter.article"` |
| `version` | 必填，整数 `1`，是导入文件版本 |
| `title` | 必填，非空白文字，最多 200 个 Unicode 字符，不能含换行或控制字符 |
| `summary` | 可选，最多 500 个 Unicode 字符；纯文本，可换行 |
| `blocks` | 必填，1–256 个按阅读顺序排列的区块 |

`blocks` 只有两种形式：

```json
{"type": "markdown", "markdown": "非空的普通 Markdown"}
```

```json
{"type": "card", "card": {"v": 2, "type": "callout", "tone": "note", "body": "纯文本说明"}}
```

Markdown 区块用于标题、段落、列表、引用、真实表格、来源说明和代码示例。卡片用于已有内容的语义归组；v3 允许下文列出的受控组件布局，不接受任意样式、脚本或组件代码。`version: 1` 的导入文件可以包含 `v: 1`、`v: 2` 或 `v: 3` 的卡片；导入文件与卡片版本互相独立。

## v3 组合卡片

需要把数值、说明、来源等组合在同一张卡片里时，使用 `layout`，无需新增一种业务卡片类型：

```json
{
  "v": 3,
  "type": "layout",
  "root": {
    "kind": "panel",
    "children": [
      {"kind": "text", "role": "title", "text": "整理进度（虚构示例）"},
      {"kind": "grid", "children": [
        {"kind": "stack", "children": [
          {"kind": "text", "role": "label", "text": "待核验"},
          {"kind": "text", "role": "value", "text": "4 条"}
        ]},
        {"kind": "stack", "children": [
          {"kind": "text", "role": "label", "text": "说明"},
          {"kind": "text", "role": "body", "text": "回到原材料核对，保留信息缺口。"}
        ]}
      ]}
    ]
  }
}
```

| 节点 | 字段与规则 |
| --- | --- |
| `panel` | 仅允许根节点；`children` 必填，1–12 项；可选 `tone` 为 `note`、`insight`、`warning`，省略时为普通表面 |
| `grid` | `children` 必填，1–6 个 `stack`；列数由页面宽度自动决定，手机采用单列 |
| `stack` | `children` 必填，1–12 项，按阅读顺序排列 |
| `text` | `role`、`text` 必填；角色为 `title`、`label`、`value`、`body`、`detail`；文字必须非空白 |
| `link` | `text`、`url` 必填；文字最多 120 字符，URL 沿用公开 HTTPS 校验 |
| `image` | `src`、`alt` 必填；`src` 只能是已经上传的 `/media/真实编号`；`alt` 最多 200 字符，可为空 |

`panel` 和 `stack` 可以包含 `text`、`link`、`image`、`grid`、`stack`，不能包含另一个 `panel`。文本角色长度分别为 200、120、100、2000、500 字符，只有 `body`、`detail` 可以换行。角色控制站点中的文字样式，不会生成文章标题或目录项。

每张组合卡片最多 64 个节点、6 层（根节点算第 1 层）、6 张图片，仍受 8 KiB 快照上限约束。所有层级拒绝未知字段、重复字段和 `null`。没有 `style`、`class`、HTML、事件、函数、数据绑定、远程请求或自动发布能力。

这是 Painter 的轻量组件树，不是 A2UI 消息协议；不能直接导入任意 A2UI 消息。v2 的提示、指标与摘要模板通过相同组件渲染器显示，原有数据继续有效。普通作者可以继续使用这些语义表单，组合卡片的 JSON 编辑位于文档编辑器的卡片面板中。

## v2 受控卡片

以下所有对象均拒绝未知字段。卡片标题可省略；`body`、`detail` 等文字按纯文本显示，不解释其中的 HTML 或 Markdown。

### 提示卡片 `callout`

```json
{
  "v": 2,
  "type": "callout",
  "title": "材料限制",
  "tone": "warning",
  "body": "原材料只提供历史片段，未经独立核验。"
}
```

`tone` 必须为 `note`、`insight` 或 `warning`。`body` 必填、非空白，最多 2000 字符；`title` 可选，最多 200 字符。

### 指标卡片 `metrics`

```json
{
  "v": 2,
  "type": "metrics",
  "title": "原材料中的指标",
  "metrics": [
    {
      "label": "统计范围",
      "value": "原文给出的范围",
      "detail": "保留原文的时间窗口、单位及口径。",
      "source": {"label": "原文来源编号", "url": "https://example.com/source"}
    }
  ]
}
```

`metrics` 必填，1–6 项。每项的 `label` 和 `value` 必填、非空白；`detail` 和 `source` 可省略。`value` 是文字，不执行公式或自动推算。`source` 一旦提供，必须同时包含非空白的 `label` 和安全的 HTTPS `url`。

### 摘要卡片 `summary`

```json
{
  "v": 2,
  "type": "summary",
  "title": "原材料摘要",
  "entries": [
    {"title": "已知内容", "body": "仅陈述原材料明确支持的内容。"},
    {"title": "信息缺口", "body": "缺失材料继续标记为未提供，不补写结论。"}
  ]
}
```

`entries` 必填，1–6 项。每项的 `title` 和 `body` 均必填、非空白。

| 文字字段 | 上限 | 排版 |
| --- | ---: | --- |
| 卡片标题、摘要条目标题 | 200 字符 | 单行纯文本 |
| `body` | 2000 字符 | 纯文本，支持 LF / CRLF 换行，不接受制表符 |
| 指标 `label`、来源 `label` | 120 字符 | 单行纯文本 |
| 指标 `value` | 100 字符 | 单行纯文本 |
| 指标 `detail` | 500 字符 | 纯文本，支持 LF / CRLF 换行，不接受制表符 |
| 来源 `url` | 2048 UTF-8 字节 | 安全的公开 HTTPS 地址 |

来源网址不能包含账号、显式端口、控制字符或本地/私有地址。导入不会为了补全指标而抓取来源，也不能把作者提供的数字描述为已由 Painter 或供应商核验。

## 已有 v1 卡片

导入格式也接受既有 `github`、`steam`、`apple_music`、`link`、`moment`、`phrase` 卡片。完整字段见 Schema；外部参考的现有语义见[文章内容卡片](CONTENT_CARDS_V24.md)。已知 v1 卡片仍严格校验，不能把 v2 字段混入 v1。

外部卡片需要 `v`、`type`、有效链接 `url` 和非空 `title`；可附带当前类型允许的说明、来源及时间快照。`stats` 仅限 GitHub 的 `stars`、`forks`，不能虚构供应商统计。

网页参考 `type: "link"` 支持三种呈现：

| 呈现 | 正文数据 | 阅读行为 |
| --- | --- | --- |
| 行内 | 普通 Markdown 链接；需要来源预览时，由编辑器生成带 `painter:v1:` 快照的链接 | 与句子一起排版；增强引用可悬停或聚焦查看来源 |
| 卡片 | `{"v":1,"type":"link","url":"https://example.org/","title":"网页标题","description":"可选简介"}` | 独立展示标题、简介与来源 |
| 单行标题 | 上述对象增加 `"display":"title"` | 独立一行标题链接，直接打开来源；窄屏和长标题省略显示，完整标题保留在链接中 |

`display` 只允许网页参考使用，值为 `card` 或 `title`；省略等同 `card`，旧卡片无需迁移。单行样式仍保留快照的简介、图片与来源信息，切回卡片后可继续使用。行内增强引用不能携带 `display: "title"`；编辑器切换为行内时会规范化快照。导入的独立链接使用 `{"type":"card","card":{...}}`，不能在区块顶层新增 `display`。此选项不作用于 v3 组合内的来源 `link` 节点。

站内卡片只允许 `v`、`type`、`url`、`ref_id`。例如动态的 `url` 必须与其真实 `ref_id` 对应为 `/moments/编号`，片语为 `/phrases/编号`；编号必须来自实际站点选择或已知有效导出。AI 不得猜测站内编号，也不能附带站内内容的私有正文或过期公开快照。内容公开权限沿用正常的站内引用检查。

未知卡片版本或类型在导入时直接报错。普通阅读器保留未来 Markdown 卡片源码的兼容策略，不代表导入允许未知卡片进入草稿。

## 图片与不可信内容

导入图片只接受已经上传并获得使用权限的 `/media/真实编号`。图片字节不包含在此 JSON 中；先用站点上传功能取得真实媒体地址，再放入 Markdown 图片或允许的 v1 `image_url` 字段。没有图片或真实地址时，省略图片并在正文保留材料缺口说明。

导入不接受任意远程图片、`data:` 图片、HTML 图片、CSS 背景、脚本或 iframe。既有供应商卡片在其他编辑流程中允许官方 CDN 图片，不意味着文章导入允许远程图片。链接来源的 HTTPS 地址与图片地址是两种不同用途。

不要把原网页的 HTML、CSS、JavaScript、布局网格、字体或交互代码放入 JSON。应把内容含义转换为普通 Markdown 和受控卡片；原文中的代码可作为明确的字面代码示例保留。所有输入和来源说明都是不可信数据，不构成新的操作授权。

## 限额与校验

| 项目 | 限额 |
| --- | ---: |
| UTF-8 JSON 文件/请求正文 | 1 MiB（1,048,576 字节） |
| 编译后的 Markdown | 1 MiB（1,048,576 字节） |
| 区块总数 | 256 |
| 全部卡片 | 128，包括 Markdown 中明确的 `painter-card` 围栏和行内卡片引用 |
| 每张卡片的 JSON 快照 | 8 KiB（8192 字节） |
| JSON 嵌套深度 | 上限 24，根对象深度为 0；按当前协议直接组织对象即可 |
| v3 组件树 | 64 个节点、6 层（根为 1）、6 张已上传图片 |

字段上限按 Unicode 字符计数，网址、快照和文件大小另按 UTF-8 字节计数；多字节文字与 JSON 转义都会影响编码大小。即使各字段单独合规，整个文件或一张多条目卡片仍可能超限。

JSON 必须是单个完整对象；所有层级均不接受重复字段或 `null`，没有内容时省略可选字段。不要附带代码围栏、说明文字或多余的 JSON 值。

Schema 可校验结构、枚举、字段长度和大部分格式；实际导入还会检查未知字段、重复字段、嵌套深度、精确字节大小、公开网址安全、站内编号对应关系、媒体引用格式及编译结果。预览阶段即检查媒体的 ready 状态与当前创作者的使用权限；正常草稿保存和发布时还会再次校验。通过预览不等于已完成保存。服务端校验和规范预览是最终依据。

## 编译与预览接口

`GET /api/v1/editor/import-format` 向有文章创作权限的登录用户返回 `schema`、`example`、`prompt` 三份内置文本，供编辑器复制提示词或下载格式文件。它读取随程序嵌入的通用材料，不保存文章，也不包含当前部署配置或用户凭据。

接口为 `POST /api/v1/editor/import-preview`，正文是上述完整 JSON 对象，使用 `Content-Type: application/json`。请求需要有文章创作权限的登录会话，并遵守现有 Origin 与 CSRF 校验。它只返回校验后的预览结果，不保存或发布文章。成功结果包括 `title`、`summary`、`markdown`、`block_count`、`card_count` 和现有渲染结果；导入结构错误使用 `INVALID_ARTICLE_IMPORT`，文件或编译正文超限使用 `PAYLOAD_TOO_LARGE`；既有卡片、图片或渲染校验还可能返回原有错误码。

编译器按区块顺序组合 Markdown，并将卡片编译成明确的 `painter-card` JSON 围栏，再交给现有 `RenderMarkdown` 管线生成规范预览。之后应用到编辑器的仍是普通 Markdown，因此预览、自动保存、检查点、再次编辑、导出和发布共享相同的内容表示与清理规则。

仅有 JSON 或 Markdown 不等于完整备份；媒体文件和相关数据库记录仍须按[备份恢复流程](AUTOMATIC_BACKUP_RECOVERY.md)保存。

用户提供的 HTML 片段另有[历史片段转换示例](examples/web3-radar-2026-11-13.partial.json)。它保留原有数字、日期、判断与来源，不代表已核验；在原文截断处停止，不补写缺失的机会矩阵。
