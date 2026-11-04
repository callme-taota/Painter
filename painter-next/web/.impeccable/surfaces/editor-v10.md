# V10 编辑页比例与布局

Mode: Operate. Existing-world refinement；保留 Painter 纸墨蓝、侧栏、真实内容、所有编辑与保存功能。本轮只改编辑页布局，不扩展账户或部署产品能力。

## Direction contract

THESIS: 标题和正文组成一个连续写作页面，文章信息作为支持区域；保存和发布在紧凑且始终可达的命令层。

OWN-WORLD: 沿用实色书写表面、细线、现有圆角和 Newsreader/sans 字体分工；不新增装饰、位图或依赖。标题与正文共享文本起点，格式工具属于写作面。

STORY: 位置 → 当前任务/保存状态与发布 → 标题/正文 → 支持性的摘要、标签、时间与封面。恢复、冲突、上传与错误仍明确可见。

FIRST VIEWPORT: 减少七条堆叠横带；桌面正文占主要宽度，右侧约280px信息栏；页面整体居中并控制最大宽度。窄屏信息折叠在标题后，正文不叠加多层大内边距。

FORM: ≥1360px文档模式为连续写作页+信息栏；其余宽度为单栏。源码模式独占工作区宽度，原编辑/预览/分屏切换保持可用。只渲染一份属性控件；DOM、标签、键盘与原保存/恢复/CAS/发布链路保持一致。

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
