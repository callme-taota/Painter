---
version: 1
slug: "web-frontend-src-components-notifications-vue"
primary_target: "web/frontend/src/components/Notifications.vue"
related_targets: ["web/frontend/src/notifications.css","web/frontend/src/components/PostIsland.vue"]
---

# Personal inbox · /me/notifications

Operate mode. Extend Painter's existing private personal space for readers, authors and the owner. The agreed first release delivers comment/reply notifications, unread count, marking individual items read and opening the actual discussion. Reference-only delivery, account isolation and current visibility are requirements. No email, new fixed tools, external services or visual-system changes.

## Direction contract

THESIS: A compact chronological inbox lets people find and resume a conversation. Use the established workbench list rather than a dashboard.

OWN-WORLD: Inherit web/DESIGN.md, existing neutral surfaces, Geist UI type, Apple-blue text links and shared button/focus tokens. Unread status uses an explicit text label, not color alone.

STORY: Enter from personal navigation or the reading links, understand the unread count, switch between all and unread, read a short excerpt, open the precise discussion or mark the item read. Hidden sources have a generic unavailable state.

FIRST VIEWPORT: Existing workbench heading and sidebar frame a narrow list. All/unread filters and refresh sit above dated rows. Each row groups actor/action, article title and excerpt; view-discussion and mark-read actions follow. Mobile wraps actions at 44px without horizontal scrolling. Errors provide retry, empty states explain what creates notifications.

FORM: Inherited personal-space list; precisely scoped local extension, code-led, no concept roll or comp needed. seed: none (unavailable).

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
