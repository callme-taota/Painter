# V11 编辑保护与资源状态

Mode: Operate. Existing-world behavioral hardening. Preserve the V10 paper/ink/blue editor, navigation and responsive layout.

## Direction contract

THESIS: A writer must see when editing or saving is safe, keep local content through failures, and explicitly confirm a conflict merge.

OWN-WORLD: Use existing notices, solid surfaces, fine rules, typography, custom controls and disabled states. No new raster assets, visual identity, runtime dependencies or database migrations.

STORY: Current task and saved state → actionable loading/error status → content → metadata → comparisons/revisions. Recovery action names explain what will be read or saved.

FIRST VIEWPORT: Loading failure and restore uncertainty appear before the writing surface. Save/publish modes are visibly locked where necessary; retry and local export remain discoverable. Preserve V10 inspector and narrow disclosure.

FORM: Initialize only after validated data; keep manual merge comparison until explicit save succeeds, including browser recovery. Lock metadata and both body editors through restore POST and fresh GET; failed/malformed reads keep the lock and local copy. Search uses native anchor activation. Media/comment actions reflect persistent state. Phrase conflict reload targets an exact owner-only record and preserves inputs on failure.

FINISH: Verify failure, retry, slow response, manual merge/reload and restore states at desktop/mobile sizes in both themes. Independent finish verdict, narrow DESIGN/PRODUCT update and evidence provenance complete the change.
