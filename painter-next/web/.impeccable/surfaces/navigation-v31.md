# V31 · Navigation and search refinement

Mode: public navigation is Operate; search is Operate. Existing home is Experience and article is Read, with their composition unchanged.

The user explicitly selected a narrow refinement of the existing design after reviewing V30: retain the current header geometry and homepage parallax, make the header a fully rounded Liquid Glass capsule containing only Painter identity and search, and give every actual right-side command its own glass lens. Other destinations move into that Dock rather than a new index page. Grouped choices may share a panel. Add one Apple-like blue for text links and navigation selection alongside the existing black/white/gray system.

The user-pinned material and capsule/circular geometry take precedence over general anti-glass or radius defaults. No replacement typography, mock device, content card system, title, cover design, business statistics or home animation is introduced.

The material is a functional foreground layer: original signed-distance rounded-edge normal maps feed an SVG backdrop displacement filter with tiny RGB channel differences; foreground content stays unfiltered. Navigation text keeps a thicker center tint. Individual Dock lenses retain an opaque-enough center and thinner refractive edge. No page screenshot, DOM clone, WebGL runtime, React dependency or idle animation is used. Chromium receives the verified refraction path. Safari/Firefox receive the CSS blur/highlight path; actual WebKit rendering is not claimed. Transparency reduction, increased contrast and unsupported backdrop filtering select solid controls.

Three persistent native browsing links lead to `/posts`, `/moments`, `/phrases`. Configured tools retain their previous scene order and permissions. Empty tool arrays clear tools while keeping browsing available. At screen heights up to 600px, the three destinations share a keyboard-enabled labelled disclosure. Touch targets remain at least 44px and menus fit the viewport. Friends remain in the conditional footer. No unimplemented library routes are invented.

Search retains the real suggestions API, request cancellation, IME handling, Ctrl/Command-K, slash, Escape, native links, arrow selection and focus restoration. Raycast-inspired grouped rows, input, icons and key hints replace the previous presentation. There are no filters in the global panel. Success status remains available to assistive technology without repeating visible group labels; loading, empty and failure states stay visible.

The V28 home stage and mapping files, V27 unframed viewer, database, provider snapshots, editorial content, admin layout and authentication flow remain outside this change. Existing home absolute-header behavior and non-home scroll-direction behavior remain authoritative. Only the old whole-rail V25 material is skipped when the new independent lenses are present.

Implementation: `frontend/src/public-chrome.css`, `liquidGlass.ts/.css`, `dock.ts`, `search.ts`, `raycast-search.css`, the public entry point and existing SSR shell. References and scoped verification are recorded in `docs/EXPERIENCE_V31.md` and ignored local `tests/artifacts/v31/`.
