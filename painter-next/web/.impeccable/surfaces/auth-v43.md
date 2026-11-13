# V43 · Account access, two-step verification and connected applications

## Scope and mode

Operate, code-led ordinary extension of the existing authentication form, personal profile and owner login-security settings. The user pinned the inherited compact shadcn appearance. The job is to control account access, protect an account and register other Web or native applications against the same identity core. Product and operational contracts remain in [PRODUCT.md](../../PRODUCT.md), [account security](../../../docs/ACCOUNT_SECURITY.md), [identity provider](../../../docs/IDENTITY_PROVIDER.md) and [backup recovery](../../../docs/AUTOMATIC_BACKUP_RECOVERY.md).

## Direction contract

**THESIS:** Make the next account-security task explicit inside the existing profile and login-security surfaces. Show the current state, its consequence and one direct action at each step.

**OWN-WORLD:** Inherit Painter's neutral light/dark shadcn workbench, shared sans type, solid paper panels, fine borders, black/white primary actions and sparse Apple-blue text links. Preserve shared 36px desktop and 44px narrow/coarse control targets and the semantic radius scale.

**STORY:** The owner sets registration and external-user login independently. An account holder confirms a local QR/manual TOTP setup, saves ten once-visible recovery codes and completes the second-step login challenge. The owner registers each connected application, then saves its Web client secret once.

**FIRST VIEWPORT:** The existing breadcrumb and task heading lead. Personal profile retains its form above the two-step verification panel. Login security places account-access switches and their save action first, then the existing GitHub panel and connected-applications panel. Public login retains the compact centered form; an active challenge replaces the credential fields with one code field, expiry text and verification action.

**FORM:** Existing task forms and compact application table; inherited surface extension, no concept tournament or new visual world. Code-led implementation. `seed: none (unavailable)`.

**FINISH:** unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Implemented states and interaction

Account access uses two independent policy choices rendered as semantic checkboxes, with one explicit save action. Closing external-user login explains the effect on reader/author sessions and connected applications while preserving owner maintenance access and public reading. Registration retains its own saved choice even while external login pauses its effective availability. Busy, saved, failed and version-conflict states are visible; a conflict offers an explicit reload.

Two-step verification first requires the current password or recent GitHub authentication. The setup panel pairs a locally generated 200px QR with a selectable manual key and six-digit confirmation field; below 600px they stack. A QR-generation failure retains the manual path. The signature interaction is progressive confirmation: setup changes no login requirement until the dynamic code succeeds, then the same panel shows the ten generated recovery codes together, with download and saved acknowledgement. Recovery codes use two columns on wider screens and one on narrow screens. Regeneration and disabling require current verification; disabling also uses the shared confirmation dialog. Leaving before saving displayed codes invokes the browser's leave protection.

The login form switches entirely to a five-minute MFA challenge after the first factor. Its focused field alternates between a dynamic code and an unused recovery code, with a restart action and explicit errors. The challenge does not represent a completed login session.

Connected applications show the real Issuer, optional integration details and a compact table of active clients. The create form uses the shared custom `SelectField` for Web, SPA and native types. Web client secrets appear only after creation or rotation, with copy and explicit saved acknowledgement; SPA/native clients have no secret. Rotation and deactivation explain invalidation through shared confirmations. Loading, empty, failed-read/retry, busy and disabled-client-count states stay visible. The table scrolls locally with keyboard focus; a `ResizeObserver` exposes the visible sideways-scroll hint only when it overflows and connects it to the region with `aria-describedby`.

## Inherited evidence and documentation boundary

Inspected `frontend/src/style.css`, `frontend/src/shadcn-theme.css`, the four auth components and `App.vue` wiring against the normative [DESIGN.md](../../DESIGN.md) frontmatter and its sidecar. The built sources reuse the shared palette, control heights, focus treatment and radius roles; QR contrast and monospace secrets are local task content, not new global tokens. No DESIGN.md or sidecar rewrite is required for this ordinary extension.

The existing sidecar's Neutral Chrome narrative predates the later blue-link refinement now recorded in DESIGN.md. That pre-existing documentation drift is reported without canonizing or repairing it, because a design-system refresh is outside this task.

## Verification boundary

The implementation sources are `frontend/src/components/AccountSecurity.vue`, `LoginPolicySettings.vue`, `IdentityClients.vue`, `AuthForm.vue` and their `App.vue` placement. Root-managed desktop/mobile light/dark captures, setup/recovery/secret states and MFA errors are under `tests/artifacts/v43/auth-ui`; generated recovery codes and client secrets are masked in saved captures. This record documents source evidence and the intended review matrix. It does not assert a completed independent UI review, release, deployment or production authentication change. Backup activation/rollback and Tunnel behavior are governed by the operational document, not by the appearance of these controls.
