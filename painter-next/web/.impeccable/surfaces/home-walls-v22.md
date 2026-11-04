# V22 configurable home walls

## Scope and mode
Extend the existing home Experience with a Steam game wall, Apple Music album wall and uploaded photographs. Operate inside one owner-only homepage manager. This is an extension of the existing neutral publication and scroll story, not a replacement visual identity.

## Job and proof
The owner chooses which parts of their interests appear: personal introduction, recent articles, games, albums, photographs and closing copy. Configure each section's visibility, order, wording and count, including the number of articles. Real stored content only. Steam can use a private server-side key to synchronize a visible game library; album links import public catalogue metadata. No personal Apple listening-history or playlist claim without the required service authorization.

## Direction
Keep the paper-plane introduction and warm scrolling surface, neutral type and existing Painter logo. Actual game art, album covers and photographs carry the new content. Games form a spacious artwork shelf, records a square-cover collection with an official Apple Music badge and destination links, photographs a quietly captioned responsive gallery. No fabricated user activity, permanent external embeds, model download or runtime third-party call on a visitor's page request.

## First view and interaction
Preserve the existing first view by default. The three empty new walls start disabled; enabled empty walls do not produce blank public sections. Order can move all six sections. Article count comes from persisted configuration. Photo links work without JavaScript and share the existing keyboard image viewer, with a home-appropriate title. Each wall has its own heading/description and optional destination link. Admin uses a compact section list/editor, a paginated wall table with explicit item edit/save, and a separate Steam configuration panel.

## Constraints and states
Home reads local snapshots only; failed upstream refresh retains the prior collection. Imports use strict IDs and fixed provider hosts, bounded requests and cached metadata; Steam keys stay encrypted and never enter public DTO/HTML. Manual edits survive later/in-flight sync. Home media refs protect deletion and GC; hidden entries stay private. Empty/loading/error/dirty/CAS/upload/retry/reduced-motion states are explicit. Existing article and moment publishing remain intact. Configured motion and background controls respect reduced-motion preferences; all content remains readable without JavaScript.

## Verification
Real PostgreSQL and fake-provider tests for defaults, permissions/CAS, sync leases/version fences, secrets, imports, SSRF, visibility and media races. Candidate browser checks exercise settings persistence, counts/order/hidden walls, manual imports/uploads and viewer behavior; one bounded desktop/mobile/light/dark screenshot batch. Formal backup, additive schema17 and before/after business/media/config checks; no candidate fixtures promoted. External account credentials are configured by the owner after delivery.
