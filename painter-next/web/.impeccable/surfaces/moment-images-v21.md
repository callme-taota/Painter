# V21 moment images

## Scope and mode
Read on the public moment feed/detail; Operate inside the existing moment manager. Add actual image attachments to the V20 content type without changing the article editor, shared navigation or the neutral visual world.

## Job and proof
Writers add up to nine ordered images, optional image descriptions and up to500 codepoints of text. Image-only moments are valid. Draft upload, saved draft and published image snapshot are distinct states. Readers see only the current published images and can inspect their originals.

## Direction
Keep the incumbent paper surfaces, shared type and36/44px controls. Photos supply the visual interest. A single picture uses a contained preview; multiple pictures form a bounded, row-major grid within each moment card. No masonry, decorative images or new service.

## First view and interaction
Reuse the existing identity/text structure and place pictures directly below content. Thumbnails remain native image links without JavaScript. Enhance them into one keyboard-accessible image viewer with count, previous/next, original link and Escape close. The manager shows pending/failed/ready uploads, removable/reorderable thumbnails and image descriptions. Do not turn uploads into a second editor mode.

## Constraints and states
Server ownership/readiness, CAS, idempotency, audit and publication snapshots apply to images. Both draft and live refs protect against deletion/GC. Changed draft pictures never replace published ones until publish; withdrawal ends their public moment visibility. Failed uploads preserve successful files and retry the same payload/key. Empty, loading, image failure, limit, disabled, conflict, dirty navigation and deleted read-only states remain legible.

## Verification
Real PostgreSQL tests for snapshot visibility, image-only, media authorization, ref deletion/GC, concurrent mutation and rollback. Candidate browser upload/failure/retry/reorder/alt/publish/withdraw paths and noJS fallbacks; one bounded desktop/mobile/light/dark visual batch. Formal upgrade adds schema16 only after backup and compares existing business/media/config data. No test fixtures promoted.
