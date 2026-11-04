# Legacy JSON package v1

`painter import-legacy --package /path/to/package --dry-run` validates a trusted, offline JSON export. The command accepts a directory containing `manifest.json`, or a JSON file. All referenced media files must be relative regular files below that file's directory. This adapter does not connect to MySQL, extract the old database, or prove a production cutover.

The executable contract is the exported `LegacyPackage` family in `internal/app/ops_import.go`. Unknown JSON fields are rejected, including password/token/session fields. IDs are JSON strings; old user/article/category/tag/comment IDs must be positive signed 64-bit integers and are preserved. Media IDs can be legacy strings and map to deterministic new opaque IDs. Timestamps are Unix milliseconds; an unknown nonpositive metadata timestamp becomes `1`. A publication timestamp must be explicitly positive.

The destination must contain no business users, articles, taxonomy, media, comments, or legacy counters. Seeded site settings/profile are allowed. Dry-run validates references, images, checksums, rendering and the empty destination, and makes no database or media changes. It does briefly acquire the maintenance gate. Import commits the database as one transaction, advances identity sequences, and does not fabricate new analytics or business events from historical activity.

Each article has a required saved `draft` and an optional `publication`. The latter must be the actual last published source, never an assumption based on the latest draft. They are rendered and stored independently. Both get immutable `import` revisions when their contents differ. A deleted article retains these historical revisions but gets no live publication pointer or search document. Historical author IDs are preserved.

Roles must be explicitly mapped to `owner`, `author`, or `reader`. An active owner requires a verified GitHub numeric subject supplied by the trusted exporter/operator; matching email addresses never links identities. Historical users without a subject remain available as authors/commenters but cannot log in. Their old passwords, OAuth tokens, sessions and login cookies are not imported. An alternative for a site with no imported owner is to configure `BOOTSTRAP_GITHUB_ID` and complete a real GitHub login after import.

Images support PNG, JPEG, GIF and WebP, at most 10 MiB and 40 million pixels. If supplied, `sha256` and `mime_type` must match the actual file. `legacy_paths` are exact internal URL paths: they create media aliases and rewrite Markdown references to `/media/{new_id}`. Absolute filesystem paths, traversal, symlinks, unsupported images, unknown references and duplicate relationships are rejected. External image sources must first be exported into the package and mapped. An explicit content `media_ids` set must exactly match the rewritten Markdown images; omit it to derive the references.

Archived taxonomy is inserted active while snapshots are constructed, then archived in the same import transaction. Historical category/tag labels are sealed into revisions. `legacy_read_count` and `legacy_counters` retain their old meanings in `legacy_counters`; they do not populate Next PV/UV tables. `legacy_counters` is a list of daily visitor estimates with one unique `source_key` and day per source. SMTP credentials and other secrets require separate operator configuration.

Media are copied and fsynced before the database commits. A private `MEDIA_DIR/imports/import-*.json` journal is created first; a `.committed` report is written after commit. Ordinary failures roll back the DB and remove newly copied objects. A power/process failure can leave orphan files or a pending journal after a successful DB commit. Inspect that journal and the database's media IDs before any cleanup; do not delete pending objects blindly. This is not a filesystem/database atomic transaction. Import refuses to append or rerun over a nonempty destination; use a fresh test destination or an explicit validated rollback.

A minimal package:

```json
{
  "version": 1,
  "batch_id": "legacy-export-2026-10-24",
  "source_system": "painter-legacy",
  "users": [
    {"id":"10","username":"historical-author","display_name":"旧作者","role":"author","github_subject":"","email":null,"phone":"","bio":"","disabled":false,"avatar_media_id":null,"created_at_ms":1700000000000,"updated_at_ms":1700000000000}
  ],
  "media": [],
  "categories": [{"id":"2","name":"随笔","description":"","archived":false,"created_at_ms":1700000000000}],
  "tags": [{"id":"3","name":"生活","description":"","archived":false,"created_at_ms":1700000000000}],
  "posts": [
    {
      "id":"100","author_id":"10",
      "draft":{"title":"正在修订的标题","summary":"未发布摘要","markdown":"未发布草稿","category_id":"2","tag_ids":["3"],"media_ids":[]},
      "publication":{"title":"已经发布的标题","summary":"公开摘要","markdown":"# 旧文章\n\n实际发布正文","category_id":"2","tag_ids":["3"],"media_ids":[]},
      "published_at_ms":1700000000000,"deleted_at_ms":null,
      "created_at_ms":1700000000000,"updated_at_ms":1701000000000,
      "legacy_read_count":125
    }
  ],
  "comments": [],
  "post_likes": [],
  "comment_likes": [],
  "bookmarks": [],
  "follows": [],
  "history": [],
  "legacy_counters": [{"source_key":"visitors-2023-11-14","day":"2023-11-14","count":12}],
  "site_profile": {"display_name":"我的博客","tagline":"记录生活","about_markdown":"关于作者","avatar_media_id":null,"social_links":[],"footer_text":"","icp_number":"","seo_description":""},
  "settings": {"registration_enabled":false,"comments_enabled":true,"collections_enabled":true,"follows_enabled":true,"analytics_enabled":true}
}
```

Relationship row shapes (all referenced IDs must exist in the same package):

| Array | Row fields |
|---|---|
| `comments` | `id, post_id, user_id, body, state` (`visible/hidden/deleted`), `created_at_ms, updated_at_ms` |
| `post_likes`, `bookmarks`, `history` | `post_id, user_id, at_ms` |
| `comment_likes` | `comment_id, user_id, at_ms` |
| `follows` | `follower_id, followed_id, at_ms` |
| `media` | `id, owner_id, path, original_name, mime_type, sha256, legacy_paths, created_at_ms` |

For backup, `painter backup --dir /new/empty/dir` uses PostgreSQL 18 `pg_dump -Fc` with an exported snapshot and a SHA-256 media manifest. The database maintenance gate is held across dump and copying, so writes/GC wait or return busy during that explicit maintenance window; public reads may continue. Use `POSTGRES_BIN_DIR` if the client tools are not on PATH. Backup directories/files are private. Application encryption/HMAC keys must be recovered through a separate protected channel.

`painter restore-check --dir /backup/dir` verifies checksums, actually restores into a random temporary database, verifies constraints, public projection and all ready-media mappings, then drops that database. Its maintenance connection needs `CREATEDB`; `MAINTENANCE_DATABASE_URL` can supply that privileged connection separately. This proves artifact restoration in an isolated database, not a production replacement, DNS/TLS restoration, key recovery, or a real legacy MySQL migration rehearsal.
