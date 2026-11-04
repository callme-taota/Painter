---
name: Painter Next
description: A personal publication with neutral reading surfaces and a compact inset workbench.
colors:
  accent: "#18181b"
  accent-soft: "#f4f4f5"
  background: "#fff"
  paper: "#fff"
  sidebar: "#fafafa"
  ink: "#18181b"
  muted: "#66666e"
  line: "#e4e4e7"
  subtle: "#f4f4f5"
  primary-ink: "#fafafa"
  focus-ring: "#52525b"
  danger: "#b42318"
  success: "#146b4d"
  dark-accent: "#fafafa"
  dark-accent-soft: "#27272a"
  dark-background: "#09090b"
  dark-paper: "#18181b"
  dark-sidebar: "#111113"
  dark-ink: "#fafafa"
  dark-muted: "#a1a1aa"
  dark-line: "#3f3f46"
  dark-subtle: "#27272a"
  dark-primary-ink: "#18181b"
  dark-focus-ring: "#a1a1aa"
  dark-danger: "#ffb4ab"
  dark-success: "#8ed9b5"
typography:
  display:
    fontFamily: 'Geist, "Noto Sans CJK SC", "PingFang SC", "Microsoft YaHei", Arial, sans-serif'
    fontSize: "clamp(3.5rem, 6.6vw, 6rem)"
    fontWeight: 600
    lineHeight: 1.05
    letterSpacing: "-0.04em"
  headline:
    fontFamily: 'Geist, "Noto Sans CJK SC", "PingFang SC", "Microsoft YaHei", Arial, sans-serif'
    fontSize: "clamp(2rem, 3.4vw, 3.25rem)"
    fontWeight: 600
    lineHeight: 1.3
    letterSpacing: "-0.04em"
  workbench-title:
    fontFamily: 'Geist, "Noto Sans CJK SC", "PingFang SC", "Microsoft YaHei", Arial, sans-serif'
    fontSize: "24px"
    fontWeight: 600
    lineHeight: "32px"
    letterSpacing: "-0.025em"
  body:
    fontFamily: 'Geist, "Noto Sans CJK SC", "PingFang SC", "Microsoft YaHei", Arial, sans-serif'
    fontSize: "16px"
    fontWeight: 400
    lineHeight: 1.6
  reading-body:
    fontFamily: 'Geist, "Noto Sans CJK SC", "PingFang SC", "Microsoft YaHei", Arial, sans-serif'
    fontSize: "18px"
    fontWeight: 400
    lineHeight: 1.85
  label:
    fontFamily: 'Geist, "Noto Sans CJK SC", "PingFang SC", "Microsoft YaHei", Arial, sans-serif'
    fontSize: "13px"
    fontWeight: 500
    lineHeight: "20px"
  source:
    fontFamily: '"SFMono-Regular", Consolas, "Noto Sans CJK SC", monospace'
    fontSize: "15px"
    fontWeight: 400
    lineHeight: 1.85
rounded:
  item: "4px"
  control: "6px"
  menu: "8px"
  surface: "12px"
  navigation: "24px"
  navigation-mobile: "20px"
spacing:
  xs: "4px"
  sm: "8px"
  inset: "12px"
  md: "16px"
  content: "20px"
  lg: "24px"
  studio: "28px"
  xl: "32px"
  section: "40px"
  chapter: "48px"
components:
  button-primary:
    backgroundColor: "{colors.accent}"
    textColor: "{colors.primary-ink}"
    rounded: "{rounded.control}"
    padding: "7px 12px"
    height: "36px"
  button-primary-dark:
    backgroundColor: "{colors.dark-accent}"
    textColor: "{colors.dark-primary-ink}"
    rounded: "{rounded.control}"
    padding: "7px 12px"
    height: "36px"
  button-secondary:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    rounded: "{rounded.control}"
    padding: "7px 12px"
    height: "36px"
  button-quiet:
    backgroundColor: "transparent"
    textColor: "{colors.ink}"
    rounded: "{rounded.control}"
    padding: "7px 12px"
    height: "36px"
  input:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    rounded: "{rounded.control}"
    padding: "7px 12px"
    height: "36px"
  chip:
    backgroundColor: "{colors.subtle}"
    textColor: "{colors.ink}"
    rounded: "{rounded.item}"
    height: "28px"
  panel:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    rounded: "{rounded.surface}"
    padding: "24px"
  navigation:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    rounded: "{rounded.navigation}"
    padding: "0 26px"
  work-navigation-active:
    backgroundColor: "{colors.subtle}"
    textColor: "{colors.ink}"
    rounded: "{rounded.control}"
  menu:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    rounded: "{rounded.menu}"
    padding: "4px"
  search-dialog:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    rounded: "{rounded.surface}"
    padding: "28px"
---

# Design System: Painter Next

## Overview

**Creative North Star: "A clear personal publication"**

Personal identity comes from the configured site name, introduction, Painter logo and saved writing. The shared visual language is neutral, compact and legible: sans typography, solid surfaces, fine borders and black/white primary actions. Public pages leave room for reading; the workbench concentrates information around the current task.

The workbench adapts the inset shell, grouped sidebar and breadcrumb hierarchy of official shadcn [dashboard-01](https://ui.shadcn.com/view/new-york-v4/dashboard-01) and [sidebar-07](https://ui.shadcn.com/view/new-york-v4/sidebar-07). This is a visual adaptation of the existing Vue components, with no React or Radix migration. Saved covers, author backgrounds and actual business data retain their own meaning.

**Key Characteristics:**

- Neutral light and dark surfaces with clear text contrast.
- One compact control and radius vocabulary across public and workbench routes.
- Solid inset workspace with restrained task hierarchy and two-level navigation.
- Readable public documents and progressive motion enhancements.

This document records the implemented visual rules. Product behavior belongs in [PRODUCT.md](PRODUCT.md); the current home composition belongs in [the V18 surface brief](.impeccable/surfaces/redesign-v18.md). It is not a visual acceptance or test report.

## Colors

The normative values are in the frontmatter and implemented by `frontend/src/shadcn-theme.css`.

### Primary

- **Ink action** (`accent`, `primary-ink`): dark action surface with light text; the dark theme reverses the pair. Use it for the main task, including save, publish and search.
- **Quiet selection** (`accent-soft`, `subtle`): low-contrast neutral fill for selected navigation, choices, tags and segments. Selection must retain an explicit semantic state.
- **Focus** (`focus-ring`): a neutral visible outline, independent of selection fill.

### Neutral

- **Page background** (`background`): white public canvas in light mode and near-black canvas in dark mode.
- **Paper** (`paper`): solid content surfaces, controls, menus, navigation and reading areas.
- **Sidebar** (`sidebar`): slightly different neutral behind the inset workspace. The studio locally maps its page background to this token; its task surface remains paper.
- **Ink and quiet ink** (`ink`, `muted`): primary content and supporting text respectively.
- **Hairline** (`line`): control strokes, table rows, panel boundaries and subtle structural separation.
- **Destructive and saved state** (`danger`, `success`): actual errors, destructive actions and confirmed saves. Color supplements text and state semantics.

**The Neutral Chrome Rule.** Shared interface chrome uses the neutral roles. Article artwork and user-selected author backgrounds remain saved content and do not define interface tokens.

The legacy `glass` variable resolves to solid paper in both themes. Public navigation, floating tools and studio surfaces use no backdrop blur.

## Typography

**Display and body font:** Geist Latin with installed Chinese sans fallbacks. The bundled Latin variable WOFF2 is 29,400 bytes, supports weights 100–900 and uses `font-display: swap`. Chinese falls back to Noto Sans CJK SC, PingFang SC or Microsoft YaHei; no CJK webfont or runtime Google Fonts request is required. Geist uses SIL Open Font License 1.1, served at `/assets/licenses/Geist-OFL.txt`; provenance is in `frontend/src/assets/Geist-SOURCE.md`.

**Source font:** SFMono-Regular / Consolas / local Chinese sans / monospace. Source editing keeps its own scale and line-number inset.

### Hierarchy

- **Display:** the configured home identity uses the display role; mobile adapts to `clamp(3rem, 14vw, 4.25rem)`. Large type belongs to publication storytelling, not routine task chrome.
- **Headline:** article titles use the headline role, with a 2rem narrow-screen size. Other public page headings use `clamp(2rem, 3.4vw, 3rem)` at 1.2 line height.
- **Workbench title:** the task heading uses the workbench-title role, becoming 22px/30px at 720px and below. Panel headings use 16px/24px; editor task chrome uses 16px/24px.
- **Reading body:** public prose and the editable document remain 18px/1.85. Public prose has a 760px maximum column.
- **Controls and choices:** 13px/20px. Ordinary editable fields use 14px/20px on desktop and 16px/20px on narrow/coarse-pointer devices. The tag picker's inline search retains 13px/20px.
- **Tables:** 13px/20px rows, 12px/20px headings and metadata, 14px/22px article links. Use size, weight and spacing together to distinguish hierarchy.

**The Task Type Rule.** Shared sans typography carries navigation, task headings, tables and forms. Keep reading and source scales independent from compact control text.

## Layout

The public content wrapper is at most 1160px, with responsive side margins. Navigation is at most 1240px. Reading prose and covers align to a 760px maximum column; the desktop author and contents rail remains separate. Mobile reserves space for the configurable right-side tools rather than overlaying text.

Desktop studio navigation is 240px wide, reduced to 216px between 961px and 1100px. The solid task surface has an 8px outer inset, a thin border and surface corners. Its breadcrumb header is 56px high. Content padding is 28px, reduced to 24px in the compact/drawer range and 16px at 720px and below. At 960px and below, navigation becomes the existing modal drawer; task content loses the desktop inset and uses the full available width. Sidebar scrolling, role filtering and active group expansion remain component behavior.

The article editor keeps one layout and one set of metadata. Its title, toolbar and body share an 850px measure and 32px internal gutter, adapting to 28px and then 20px. At 1360px and above the shared inspector is 280px wide; narrower layouts use the same article-information disclosure. Source and document views replace only the body representation. Horizontal tables and source formatting groups scroll locally when necessary.

The V17 density contract remains normative: ordinary controls and single-line options target 36px on desktop and 44px at widths up to 720px or with a coarse pointer. Nested tags and segments use 28px/36px. Selected tags do not add a second vertical padding layer; multiline content may grow rather than clip. Checkbox/radio marks are 16px. Purposeful reading links, information buttons and floating actions may retain larger touch targets.

## Elevation & Depth

Fine borders and neutral surface changes carry most depth. The studio task surface, sidebar and panels are flat; popovers use one small shadow. Public controls may use the smallest shadow. Navigation and floating tools are solid, with no glass treatment.

### Shadow Vocabulary

- **Small control:** `0 1px 2px rgb(0 0 0 / .05)`; dark mode uses `.2` opacity.
- **Popover:** `0 4px 14px rgb(0 0 0 / .09)`; dark mode uses `0 4px 16px rgb(0 0 0 / .24)`.
- **Search dialog:** `0 12px 36px rgb(0 0 0 / .16)` above an unblurred dim backdrop.

The home paper illustration has its own artwork shadow. It is not a reusable elevation level for forms or panels.

## Shapes

Central `style.css` variables own the radius scale: items/tags 4px, fields/buttons 6px, menus 8px and containers 12px. Public navigation retains its established 24px desktop / 20px mobile silhouette. Avatars and radio marks stay circular. The fullscreen mobile task surface has square outer corners; its inner components retain their normal radii.

**The Shared Shape Rule.** Reuse the semantic radius token for the element's role. Component-specific values must not create an independent control scale.

## Components

### Buttons and fields

Primary actions use the ink/primary-ink pair, secondary actions use paper and a fine stroke, and quiet actions use transparent surfaces. Hover changes the neutral fill; focus remains an explicit outline. Studio buttons and inputs omit ambient shadows. Shared `SelectField` menus are custom-rendered, solid and bordered, including teleported menus. Native file and date pickers remain available for device integration.

### Choices, tags and segments

Semantic checkbox/radio inputs retain their accessible meaning under project rendering. Selected tags and body-view segments use the inset height and item radius. The author palette is a compact swatch, name and radio on one row; small screens use two columns. Menus retain keyboard movement, Escape dismissal, disabled states and viewport-aware positioning.

### Panels and tables

Studio panels use paper, a hairline border and surface corners, usually with 24px padding and 20px on narrow screens. Management tables use restrained neutral headers, clear row boundaries and local horizontal scrolling. Per-row menus escape table clipping. Metrics describe real data rather than decorative dashboard examples.

### Navigation and search

The public header keeps Painter identity, articles, moments, phrases and search. Search shortcut hints appear on hover or keyboard focus; filters remain on the full search page. The footer remains centered and compact. Non-home navigation returns on upward scrolling and hides on downward scrolling while preserving document space; focus, menus and dialogs hold it visible.

The workbench uses the existing two-level grouped sidebar and a breadcrumb inside its task surface. Active destinations use a neutral fill and ink text. A modal drawer serves narrow screens. The public header, footer and right action rail do not appear inside the workbench.

The optional create action uses the existing right rail and a solid, compact disclosure menu with article and moment destinations. Scene settings own its visibility; the new action starts off and does not reset existing selections. Only writers see it. Its shape, focus and touch targets reuse the shared control/menu roles rather than adding a second floating button system.

### Short moments (V20)

Moment cards use solid paper, a fine border and 12px surface corners. A compact author/avatar row and publication time precede the complete plain-text body. Body text uses the shared sans family at 16px/1.85 (15px on narrow screens), preserves whitespace and wraps long uninterrupted text. Real author identity and content carry the variation; cards have no decorative icons, gradients or metrics.

The moment feed has its own list/two/multi segmented choice, with explicit aria-pressed state and the shared 36px/44px control targets. Two columns are the default; list constrains the measure to 760px, multi uses three columns or four on sufficiently wide screens. At 720px and below every mode is one column without rewriting the saved preference. Grid reads across rows in DOM order; do not use masonry, CSS columns or these selectors for article lists. Hide enhancement-only layout controls without JavaScript.

Moment management pairs a quiet paginated list with one composer. The composer comes first in DOM and narrow-screen order; desktop places it beside the list. Empty, loading, failed, saved, unpublished-change, conflict and deleted states use actual text and the shared semantic roles. Character counts are tabular and count Unicode code points, including spaces and line breaks. Save-draft and publish controls remain separate, and a conflict comparison exposes the latest server text without silently replacing local input. Scope and behavior are documented in [V20](../docs/EXPERIENCE_V20.md); this pattern record does not assert browser acceptance.

### Moment images (V21)

Optional photos extend the existing moment card below its plain-text content. A single image is contained so its complete composition remains visible; groups use one, two or three columns by count, with four images arranged two-by-two. Image order follows saved DOM order. These internal photo groups are independent of the feed's list/two/multi card preference. Original-image anchors remain usable without JavaScript. Gallery thumbnails keep their existing neutral surfaces and radius/focus roles.

The shared viewer (V27) places the complete, uncropped image directly on a dark fullscreen backdrop in either theme. The image has no frame, card, border, rounded corners, shadow, header, footer or visible caption. Normal multi-image viewing shows only close and previous/next icons; a single image shows only close. Title, position/count and alt remain available to assistive technology. Original-link recovery appears only after a failed load. Preserve native dialog modality, Escape dismissal, focus return and keyboard navigation; this same treatment serves moments, home photos and ordinary article images. Scope and evidence are recorded in [V27](../docs/EXPERIENCE_V27.md) and [image-viewer-v27.md](.impeccable/surfaces/image-viewer-v27.md).

The composer uses compact image rows with thumbnails, real per-file upload states and explicit move/remove controls. Optional descriptions live in native details with an existing-text summary, a full labelled field and a Unicode counter; invalid lengths expose the field for correction. Retain the 36px/44px control targets and shared radius tokens. Failed uploads remain visible and retryable without discarding successful items, and conflict comparisons show the server's full image order and descriptions. Loading or failure must never silently replace local images. Implementation boundaries and pending acceptance are recorded in [V21](../docs/EXPERIENCE_V21.md).

### Continuous article editor

Direct document writing is primary. More exposes Markdown source and export, and source preview/split remain secondary tools. Title, summary, tags, cover and date controls keep the same DOM and location across representations. Focus marks the title underline or document formatting divider; it does not draw a large border around the writing canvas. Source line numbers never overlap the first Markdown glyph.

### Personal home story

V18 replaces the old WebGL torus and `homeLens` renderer with layered DOM paper pages. They show the configured introduction name (falling back to site name), tagline and Painter mark. Native scrolling drives a small fan/tilt and three content scenes; events schedule at most one pending animation frame, with no idle render loop, model download or GPU canvas. Reduced motion and no JavaScript expose readable normal-flow scenes. Recent writing uses real saved covers and links, with native horizontal access on mobile/static layouts. This surface-specific composition is not a template for management screens.

The V19 refinement adds a home-only scrolling canvas: light mode blends from pure white to book-paper cream (#f2eadc), while dark mode blends from its near-black base to warm charcoal (#211e19). `--home-warmth` follows smooth native story progress from0 to1, and CSS resolves `--home-surface` from the current theme. Body, sticky story and closing section use the same resolved surface so later writing and footer do not jump back to gray. Returning to the top restores the base; reduced motion and no JavaScript keep the static theme base. This does not change global background/paper tokens, article art or other routes.

### Configurable home walls (V22)

Extend the existing home composition with saved game artwork, square album covers and quietly captioned personal photographs. Each wall has its own heading, description and optional destination; the six home sections may move or be hidden without creating a second visual world. Defaults preserve the existing first view and six-article chapter. New empty walls start off; enabled walls without visible content do not render empty headings. Actual content carries the color, while chrome remains neutral. The official Apple Music mark links to a real album destination; it does not decorate unrelated cards or imply an account connection.

Photographs form a responsive gallery with native original-image links and the existing keyboard image viewer. Saved ordering follows DOM reading order. Configured motion and warmth remain progressive enhancements and respect reduced-motion preferences. All-disabled layouts retain the shared header/footer and accessible page identity.

The owner manager uses three task tabs, a compact section order list with one active editor, a paginated wall table with explicit item edit/save, and a separate Steam configuration panel. Native controls and shared SelectField keep the 36px/44px and 4px/6px/8px/12px contracts. Upload queues expose real state, preserve successful files and require photo review before public visibility. Empty libraries, unconfigured services, upstream failures, dirty forms and CAS conflicts remain explicit. Scope and evidence are recorded in [V22](../docs/EXPERIENCE_V22.md); the surface direction is [home-walls-v22.md](.impeccable/surfaces/home-walls-v22.md).

### Immersive games and touching records (V23)

Extend the same homepage with a rectangular scroll aperture into actual Steam hero art. At most five full-screen game identities carry official wordmarks or the real title and truthful provider hours; media provides the dark photographic surface in either theme. Native scrolling drives entry, scene depth and exit without a wheel trap or idle loop. An adjacent album contact sheet sits beneath the closing mask so the transition has no blank intermission. Nonadjacent ordering keeps both walls independent. The configured introduction, article and photo patterns remain intact; global colors, type and control tokens do not change.

The record grid has touching square covers and a restrained source/caption edge, including an honest partial-playlist notice. Complete /games uses a responsive multi-column library, ordinary sort links and native pagination. Reduced motion, motion-off and no JavaScript retain normal document flow and native destinations. Missing game wordmarks use readable saved titles; unknown time stays visibly unknown rather than zero.

Operate stays in the existing three tabs: compact read-only provider hours and an optional native art disclosure, a fixed /games destination, and a bounded playlist preview with separate track/album/shown counts. The first 24 covers scroll within the preview; explicit save and retry actions remain beside truthful visibility and partial-read copy. Failed reads retain the previous preview; dirty forms, CAS and uncertain acknowledgements preserve input and require a clear next action. This is a task extension, not a second dashboard style. The authorized composition is [game-records-v23.md](.impeccable/surfaces/game-records-v23.md); current acceptance limits are in [V23](../docs/EXPERIENCE_V23.md).

### Collected worlds and publication activity (V24)

The complete game library lets saved artwork lead: large 2:3 portrait covers, shared two-line title measures and aligned tabular hours. Its standard three-column collection becomes four on the widest screens, two at intermediate widths and one on narrow phones. Surface corners, ink, muted metadata and focus treatment reuse the shared roles. The native destination is apparent on focus and always visible on phones; existing sorting and pagination stay compact above the collection.

The publication calendar is a restrained grid of 12px day cells, organized in seven-day weeks with 5px gaps. A component-local four-level green scale expresses actual day counts in both themes; it is a data-visualization role, separate from global success or interface selection. Empty days use subtle/line, future days are subdued and out-of-range padding is hidden. Exact date, count, source and timezone explain the color. Keep overflow inside the calendar, initially expose its latest nonfuture date, and retain the visible sideways-scroll instruction in the public view and owner preview.

Project links form a visible hand of tall neutral cards, using paper/ink, surface corners and real repository content. The active card straightens and rises while neighbors move aside; hover, focus and touch share the same item state. A soft card shadow is specific to this overlapping composition, rather than a new form-panel elevation. Narrow screens retain the hand within a native horizontal scroller. Reduced motion removes the transforms and overlap so the links remain a readable collection.

### Article references and shared images (V24)

External-content cards are quiet bordered surfaces within the existing reading measure. Shared paper, ink, muted text, surface corners and visible focus frame saved content. Titles use 17px/25px and supporting text 14px/22px on desktop, adapting to 15px/23px and 13px/21px on phones. GitHub uses a compact avatar, Steam gives its artwork a wide region and stacks art above full-width text at 720px and below, while Apple Music retains its saved square cover. A provider's low-resolution source remains an honest content limitation rather than a reason to invent an asset.

Inline references remain underlined prose links, including author-supplied Emoji. Their compact source preview uses the existing menu radius, paper and popover depth; it moves within the viewport, supports hover/focus and an explicit first-touch inspection state, and keeps the original target available. The insertion panel reuses ordinary fields, controls, error text and server preview inside the continuous editor. Uploaded article images reuse the existing single keyboard image dialog; do not add a separate viewer style or treat provider card artwork as gallery items.

These are scoped extensions of the existing system. Their composition and behavior are recorded in [home-and-cards-v24.md](.impeccable/surfaces/home-and-cards-v24.md); evidence and release limits belong in [V24](../docs/EXPERIENCE_V24.md). They do not change the normative frontmatter, shared identity or control-density rules.

### 首页影像上的浮动工具（V25）

首页游戏与音乐影像后的右侧工具是上述实色浮动工具规则的局部例外：材质随工具栏处实际影像覆盖渐变，计算包含游戏圆角遮罩与音乐区域的覆盖并集。保留原有几何、图标、焦点与实色菜单；离开影像恢复原有纸面、边线和小阴影。背景模糊限定0–24px、饱和度1–1.65，以220ms缓动。减少透明度、提高对比度、无 JavaScript或能力不支持时保留实色；减少动态效果仅移除缓动。此例外不建立全局玻璃 token，也不扩展到导航、工作台或其他页面。边界见[表面记录](.impeccable/surfaces/home-controls-v25.md)，验收见[V25](../docs/EXPERIENCE_V25.md)；既有 frontmatter 与 sidecar 保持原样。

### 首页连续手势（V26）

首页可在既有纸页、项目牌组、发布日历和收藏影像之间延续同一个滚动手势：纸页收向真实项目，较小纸片落向真实日期格，游戏影像从实际测量的一格唱片位置展开。分组依赖实际渲染后的相邻区块，保留站长顺序；这是首页的局部构图，不改变系统 token。退出游戏先淡出身份与控件，再收小影像；持有原生焦点时保持对应画面完整，焦点离开后恢复当前滚动状态。V25 玻璃按四边非对称遮罩的实际覆盖延续；静态文档流和偏好回退保留。范围见[表面记录](.impeccable/surfaces/home-journey-v26.md)，证据与发布边界见[V26](../docs/EXPERIENCE_V26.md)。

## Do's and Don'ts

### Do:

- **Do** use the shared neutral tokens in both themes, including the sidebar/paper distinction.
- **Do** preserve the 36px/44px control targets, 28px/36px inset elements and semantic radius scale.
- **Do** keep real content, accessible states, keyboard focus and local overflow behavior visible.
- **Do** keep reading, source and task typography distinct within the shared sans family.
- **Do** preserve static reading and reduced-motion fallbacks when adding enhancement.

### Don't:

- **Don't** reintroduce blue selected capsules, glass task surfaces or the former serif display system.
- **Don't** migrate frameworks or add a component runtime merely to imitate the reference appearance.
- **Don't** resize controls by adding nested vertical padding, clip choice text or allow page-wide horizontal overflow.
- **Don't** turn article artwork, user backgrounds or home motion into global interface tokens.
- **Don't** imply that a visual rewrite changes permissions, settings, data or publication behavior.
