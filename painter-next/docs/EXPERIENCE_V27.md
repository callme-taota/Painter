# V27：无框图片查看器

动态、首页照片与普通正文图片继续共用一个查看器：深色全屏遮罩上直接显示完整、未裁切的图片，浅深主题一致。移除卡片外壳、边框、圆角、阴影、标题栏、页脚与可见说明；正常多图只显示关闭、上一张、下一张图标，单图只显示关闭。标题、序号与 alt 仍供辅助技术读取，只有加载失败才显示“打开原图”。

保留原生模态、Escape、Tab 范围、焦点返回及方向键/Home/End；点击背景关闭，点击图片保留。修饰键、无 JavaScript 或不支持 dialog 时沿用原生链接。每次选择独立持有加载请求，旧响应不能覆盖新图或重新打开已关闭窗口。缩略图网格和提供方卡片目的链接保持。实现见[共享查看器](../web/frontend/src/momentGallery.ts)，局部约束见[表面记录](../web/.impeccable/surfaces/image-viewer-v27.md)。

[隔离报告](../tests/artifacts/v27/viewer/report.json)60/60项通过；真实 CC0 湖景照片等待解码后捕获，桌面/移动、浅/深主题四张最终截图均经[逐张审阅](../tests/artifacts/v27/viewer/visual-review.json)。[独立收尾](../tests/artifacts/v27/finish-review.md)的 SHIP 仅限该隔离范围，行为结论由测试报告支持。[唯一一次检测](../tests/artifacts/v27/detector.json)为零项发现，未重跑。初次未等待解码的截图不计入验收；[照片来源](../tests/browser/fixtures/moments/README.md)仅供隔离测试，未导入正式内容。前端与 Go 构建通过。

**已更新局域网 <http://192.168.2.209:18080>。** [实际候选集成](../tests/artifacts/v27/actual-integration.json)11/11项通过，覆盖三类图片入口、共享查看器与焦点返回；[正式只读核验](../tests/artifacts/v27/formal-smoke.json)14/14项通过，确认公开路由、6份当前资源及 V26 首页顺序，运行错误与业务写入均为零。正式所查页面没有公开图片，打开图片数为0；正式核验范围为交付、路由和首页顺序，图片交互与视觉依据分别来自候选和隔离证据。[发布记录](../tests/artifacts/v27/promotion.json)确认仅替换应用二进制，58张表逐表行摘要、媒体、环境配置与迁移账本一致，schema 仍为20；未迁移、恢复数据库或复制候选内容。
