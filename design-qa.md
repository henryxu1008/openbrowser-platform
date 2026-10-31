# Store workbench integration — visual QA

## Scope and source
- Selected visual: option 1, light-blue Store list workbench.
- Source reference: `public/reference.png` from the supplied prototype source archive, version 3 (1487 × 1058 pixels). The reference image was opened and inspected.
- Supplied self-contained HTML: `shuomi-browser-prototype.html`, SHA256 `73e4a8ca885f99540cd1d3b1e58872a6da6315b1e5d6c7be9aeb72fcc0c109ae`. Its source and CSS were inspected; rendered HTML verification was not completed.
- Implementation: existing Store workspace/list/detail and managed environment editor, not a new application shell.

## Findings and intentional boundaries
- [P1 / verification] Browser-rendered implementation screenshot unavailable. The isolated preview did not become reachable by the review browser. No screenshot, viewport, pixel-density comparison, console check or browser interaction pass is claimed.
- Full-view and focused-region comparison: NOT_RUN. Source reference pixels are not a substitute for a rendered implementation capture.
- Typography: existing production font stack and shared components preserved; title hierarchy and table rhythm adapted from the reference. Rendered wrapping and density NOT_RUN.
- Spacing/layout: scoped list/filter/row styles and narrow-screen wrapping added; existing app navigation retained. Responsive visual verification NOT_RUN.
- Colors/tokens: blue accent scoped to Store workspace; existing semantic theme backgrounds/text retained. Dark-theme contrast verification NOT_RUN.
- Assets: no prototype-generated brand or platform artwork copied into production; existing product identity remains unchanged.
- Copy/content: only authorized catalog facts shown. No invented running/network-ready counts, last-used times or installed kernel versions. Runtime state remains in the existing detail read.
- Existing network means this Store's saved local configuration only. The current `direct_or_local_only` contract does not expose a global proxy pool or paid inventory. Custom empty input cannot silently become direct; direct is an explicit choice (or the explicitly labelled saved configuration).

## Prototype version 4 palette update
- Latest supplied HTML/source archive version 4 was separately materialized and compared byte-for-byte against retained version 3. HTML SHA256: `3a61a3c2386d264dabe7401835d4f6784fd22c73dc0f37db71315cf865a56efd` (521087 bytes).
- Source delta: new `src/theme.css`, its import in `src/main.jsx`, updated guidance/QA and generated assets. Business handlers, policy functions, dependencies and the original 18 prototype tests are unchanged.
- Applicable corrections are restricted to `[data-theme='light'] .store-workbench`: darker secondary/helper labels, clearer borders, explicit disabled button colors and preserved opacity. Native control scheme follows the production dark/light/cream/mint/ocean attribute; non-light palettes and other application pages are not replaced.
- Four CSS contract tests first failed on the previous candidate and now pass. They verify scope, declared native schemes and selected literal foreground/background contrast pairs, not computed browser colors or full accessibility compliance.
- Existing shared Form error borders are preserved. Body-mounted modal portals do not inherit the scoped palette; no modal-wide correction is claimed.
- The prototype author's seven-page visual claims are not independent evidence for this production candidate. Browser-rendered comparison and usability gates remain NOT_RUN; no browser policy retry was made.

## Evidence and next gate
- Offline real-component tests exercise the Store adapters and generated bridge with only native/events/renderer boundaries mocked. They are not visual or native-process acceptance.
- Before visual approval: render source HTML and candidate at matching viewport/state; capture list, detail, network editor, loading/error and narrow layouts; inspect console and interactions; compare both full view and focused regions; fix any P0/P1/P2 differences and record the revised captures.
- No completed visual iteration is claimed. Keep the PR draft.
- Native Windows/customer Store/CDN lifecycle acceptance: NOT_RUN. No real login, Store open, proxy request, procurement, payment, deployment or release was performed.

final result: blocked
