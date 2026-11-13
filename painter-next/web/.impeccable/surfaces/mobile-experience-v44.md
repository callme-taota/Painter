# V44 · 同一 Painter，按端组织任务

## Scope and mode

本轮是既有视觉世界内的跨端交互改造，采用 code-led 实现，不替换品牌或生成新的视觉方向。用户已要求保留设计原则，由桌面、平板、手机分别采用适合输入方式和空间的任务路径；本子任务按该明确 brief 记录源码，不另发设计访谈。

- **Experience**：首页、项目/唱片/游戏/照片收藏；手机将桌面连续滚动投影改为自然内容流与触摸集合
- **Read**：文章列表、正文、作者、动态与片语；正文与真实内容优先，工具按需展开
- **Operate**：全局搜索、结果筛选、工作台、文章编辑、个人设置和登录安全；减少并行任务，明确下一步与返回

完整合同：[全端交互规范](../../../docs/CROSS_PLATFORM_EXPERIENCE.md)。身份、备份、权限、数据写入与公开版本合同沿用 [PRODUCT.md](../../PRODUCT.md)、[账号安全](../../../docs/ACCOUNT_SECURITY.md) 与 [身份服务](../../../docs/IDENTITY_PROVIDER.md)。当前仍是 Web，不宣称原生 App、离线应用或完整客户端业务令牌授权已经实现。

## Direction contract

**THESIS:** 一个品牌、同一套资源与权限，根据端重新组织任务。手机先让用户知道自己在哪里、此刻在做什么、如何完成和退出，而不是把桌面列数压成一列。

**OWN-WORLD:** 保留原 Painter logo、既有中性深浅色与无衬线字体、黑白主要动作、正文和明确阅读动作的苹果蓝、公共导航与搜索的 Liquid Glass。工作表单、文章属性和正文仍是实色纸面；手机导航外壳的局部玻璃不扩散到整个后台。共用控制、菜单、表面的语义半径与44px触摸目标，组件局部导航形状不重写全局 token。

**STORY:** 手机上从首页/文章/动态进入内容，在更多中找到片语、游戏和页面工具；搜索独占一屏，筛选暂存后显式应用。工作台用内容/新建/我的/全部承接跨任务跳转，进入编辑后收起跨任务底栏，以正文→文章信息→明确发布完成一篇稿件。个人和登录安全每次只展开一个分组，未保存表单保留挂载。

**FIRST VIEWPORT:** 手机公共页保留 logo/搜索顶部与四项底部导航，内容从清楚的标题和可读正文开始，右侧桌面 Dock 退场。工作台顶部是返回、当前任务标题和全部功能；非编辑页底部是角色适配的常用任务。编辑页显示保存状态、正文/文章信息两步骤和直接可达的保存/下一步动作。平板保留适合触摸的紧凑抽屉和阅读布局；桌面保留完整侧栏、表格、检查器与滚动叙事。

**FORM:** code-led 普通扩展，`seed: none (unavailable)`。共享SSR路由、输入和业务状态，以分端壳、原生dialog、任务tabs与CSS改变组织；不复制数据库、API、表单或编辑草稿。signature interaction 是手机编辑器“正文→文章信息”的单稿两任务：下一步只转到信息检查，真正发布仍需显式确认操作。临时任务以Back关闭、Forward恢复为历史目标，焦点与滚动随关闭归还。

## 按端的实现范围

| 端 | 公共内容与导航 | 操作与创作 |
| --- | --- | --- |
| 手机 `≤720px` | 首页/文章/动态/更多底栏；更多底部面板；全屏搜索；筛选底部面板；自然流首页、触摸横向集合和单列阅读 | 任务标题栏；内容/新建/我的/全部；读者首项为收藏且无新建；文章正文/文章信息；个人资料/主页/安全；登录安全访问控制/GitHub/应用接入 |
| 平板 `721–1023px` | 不强制套用手机底栏；保留公共顶部和既有Dock，触摸入口仍完整可用 | 工作台960px内容断点内使用原抽屉，961px起可用既有侧栏；属性在1360px前按需展开 |
| 桌面 `≥1024px` | logo/搜索顶部、右侧独立玻璃工具、Raycast浮层；首页原有连续滚动表达 | 分组侧栏、比较型表格、1360px起文章检查器；手机任务分组在宽屏仍显示原完整表面 |

720px是手机交互壳分界，960px/1360px是现有工作台和检查器的内容容量分界；不为了统一数字制造拥挤布局。切宽度只改变表示，不创建另一份文章或更新用户保存的动态布局偏好。

## 公共任务、材料与帮助函数

[`mobileNavigation.ts`](../../frontend/src/mobileNavigation.ts) 在手机将**同一组**场景配置工具从桌面Dock移到更多面板，回到宽屏时放回原位置；角色授权、主题、可选创作、点赞和顶部/底部动作沿用现有来源。更多提供片语、游戏与登录/我的空间，文章目录从原目录链接构建折叠入口；标签仍在搜索，About和友链仍在页脚。

[`mobileOverlays.ts`](../../frontend/src/mobileOverlays.ts) 负责临时手机任务的`painterMobileOverlay`历史命名空间、拥有者标记、关闭与导航/提交的先后关系。`registerMobileOverlay`让已接入dialog参加Back/Forward；`dismissMobileOverlay`等待历史关闭再执行同源链接或表单导航；`initMobileViewport`将可视高度/顶部和可编辑焦点结合，为键盘与弹层提供布局提示。重载清除无法恢复的临时内存任务标记。这里记录实现责任，浏览器边界行为需由本轮QA确认。

[`search.ts`](../../frontend/src/search.ts) 保留真实建议、请求取消、IME、键盘与焦点恢复；手机布局变为整屏玻璃任务，隐藏桌面快捷键行。Enter与建议链接仍应完成相同的真实导航，不能被历史清理吞掉。普通`/search`与SSR表单仍是增强失败时的入口。

[`searchFilters.ts`](../../frontend/src/searchFilters.ts) 不复制筛选值：把同一组标签或排序输入移入sheet，打开时保存选择快照；取消还原，重置只改当前暂存，查看结果/提交先退出历史层再提交。手机面板一个内容滚动区、固定标题和动作区；全局建议框继续不承担筛选。

[`mobileChrome.css`](../../frontend/src/mobileChrome.css) 定义公共底栏、更多/筛选面板、全屏搜索、低高度与键盘提示、safe-area和实色回退；原生dialog负责模态焦点，背景滚动受控。底部导航预留正文空间；登录/注册与检测到输入键盘时收起无关公共底栏。浏览器不支持或无JS时保持既有原生内容入口，不显示空的手机工具壳。

## 内容与工作台的实际组织

[`mobileContent.css`](../../frontend/src/mobileContent.css) 将手机首页的介绍、Painter卡片、项目、真实发布日历、唱片、游戏和最近文章改为正常纵向章节；项目、唱片、游戏与最近文章采用局部触摸横向集合，保留可见下一项、标题与原链接。手机不启用桌面长距离pin、投影桥接或磁吸；热力图补充44px前后日控制，不要求点中小色块。相邻真实游戏格仍保留其身份，不替换专辑或伪造游戏数量。桌面连贯过渡保留。

文章列表封面/日期/标题逐项对齐，正文保持单列阅读宽度，作者身份靠近文章引导区，时间仍从`i`展开。代码、宽表格只在自身范围横滚；推荐区与正文/评论有独立边界。作者头像/名称/关注同排，简介和作者内搜索在其下。动态手机单列，媒体保留自己的图片分组与既有无框预览。

[`App.vue`](../../frontend/src/App.vue)、[`StudioSidebar.vue`](../../frontend/src/components/StudioSidebar.vue) 与 [`mobileWorkbench.css`](../../frontend/src/mobileWorkbench.css) 组织手机任务标题、角色对应的底部任务和分组“全部功能”sheet；工作台不混入公共footer/Dock。窄屏任务tab使用语义选中态、方向键/Home/End；资料/主页/安全和访问控制/GitHub/应用接入以`v-show`保留已挂载组件及未保存字段。`#account-security-title`仍能直接到账号安全。

[`PostManager.vue`](../../frontend/src/components/PostManager.vue) 与 [`TagManager.vue`](../../frontend/src/components/TagManager.vue) 保留真实表格资源/字段语义，手机按任务行显示标题、状态、时间与编辑/更多；不要求先横向滚到最后一列。比较性数据表仍可局部横滚。

[`MarkdownEditor.vue`](../../frontend/src/components/MarkdownEditor.vue) 以正文/文章信息两个可切换任务保留同一editor和draft；属性、封面和日期不会另存一份。正文下的下一步先检查信息，源码、导出、检查点与撤回放在更多；手机不显示并排源码预览选项。软键盘检测时收起底部固定动作，保留正文滚动与系统编辑；实际iOS/Android键盘几何需真机复核。CAS、只读锁、IME、IndexedDB恢复与服务端保存确认仍由原有逻辑负责。

## 来源、状态与验证边界

本记录是局部适配，未新增或替换shipping位图、字体、logo或内容媒体，没有新增图像生成资产；这些资产沿用原来源和既有追踪。源码中的手机规则复用`--paper`、`--ink`、`--muted`、`--line`、`--link`及语义半径；没有改全局frontmatter或sidecar，旧设计叙述漂移不在此轮顺带修复。

本轮待执行/核对的浏览器证据入口：

- [`v44-mobile-chrome.cjs`](../../../tests/browser/v44-mobile-chrome.cjs)：公共壳、搜索/筛选、主题和视口
- [`v44-mobile-history.cjs`](../../../tests/browser/v44-mobile-history.cjs)：临时任务Back/Forward、同源导航与提交
- [`v44-mobile-content.cjs`](../../../tests/browser/v44-mobile-content.cjs)：首页、阅读、作者和收藏内容
- [`v44-mobile-workbench.cjs`](../../../tests/browser/v44-mobile-workbench.cjs)：任务tab、列表、编辑与角色

文档编写时仅核对源码，不提前宣布上述检查通过、无设计问题或已经部署。测试结论、截图、修复评分和发布信息以root最终发布记录为准。真实iPhone Safari、Android软键盘、系统边缘返回/缩放、VoiceOver/TalkBack、设备资源与弱网性能尚需单独设备证据；Chromium设备模拟或合成触摸不替代这些验证。
