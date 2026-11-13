# V37 · Liquid Glass search

Mode: Operate. The user explicitly requests that search use the same Liquid Glass material as the existing public navigation. Preserve the Raycast composition, real suggestions API, keyboard/IME behavior, focus restoration, native links and full-search GET form. No global-panel filters, fabricated published content or new search features.

Register the dialog and full-search keyword/filter controls with the existing bounded lens renderer. One optical layer covers the command panel; input/status/footer stay fixed while only suggestions scroll. The foreground remains unfiltered. Use stronger backdrop softening for large textual overlays, retain visible selected rows and blue links, and keep dark tint strong enough for bright underlying artwork. Touch submit/close targets reach 44px.

Keep public result text and the rest of the page solid. Native no-JS browsing/search remains usable. Reduced transparency, increased contrast and unsupported backdrop filtering retain a solid surface; supported non-refraction renderers use CSS blur/highlights. Do not claim actual WebKit testing from a Chromium fallback-branch test.

Evidence and release scope: ../../../docs/EXPERIENCE_V37.md. Local before/after screenshots and anonymous read-only browser reports are under tests/artifacts/v37/.
