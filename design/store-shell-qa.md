# V4 shell and integration QA — local candidate

This review covers the signed-in shell, placement of existing enterprise/account controls, Store filter layout and narrow navigation. It does not approve every business screen or the native customer lifecycle. Related to #21; no public PR or existing PR revision is made by this local work.

## Visual truth and normalization

- Authoritative source: approved option one, v4 HTML dated 2026-10-31T10:40:35Z, SHA256 `3a61a3c2386d264dabe7401835d4f6784fd22c73dc0f37db71315cf865a56efd`, 521087 bytes. Reverified, rendered and actually viewed.
- The source's original 52px brand PNG is reused unchanged. Extracted asset SHA256: `fbacd5cbcdb4ee2987cb25349a06f717952a621010415010e116d62fcc56d067`.
- Implementation: actual AuthGate, BusinessShell, Layout, routes, Store pages, editor, shared modals and generated bridge. Only native responses/events are synthetic. The original frontend operation logger remains active. No real login, proxy, kernel, Store operation or external service is used.
- Desktop source/candidate CSS viewport and screenshot: 1280 × 900, density 1. Additional candidate widths: 1200 and 1440. Narrow captures: 390 × 900, density 1.
- Source and candidate have different business records. The source contains simulated procurement, runtime/network readiness, team roles and commerce actions; these are not copied into production. Comparison of shell geometry, type, palette and assets is meaningful; identical business-state/table screenshots are not claimed.
- At 390px the source retains a 760px minimum page width. The candidate instead preserves its shell language in a keyboard-accessible drawer and keeps content at 390px. This is a responsive adaptation, not 1:1 reproduction of source overflow.
- Original PNGs are unchanged. Comparison canvases add 34px captions without scaling: full 2560 × 934, sidebar 720 × 934, header 2080 × 239, narrow 1170 × 934. The fixture-only debug overlay is hidden before final screenshots; route/call evidence remains in JSON.

## Evidence opened together

- [Full source/candidate](store-shell-evidence/source-candidate-full.png).
- [Focused source/before/after sidebar](store-shell-evidence/source-before-after-sidebar.png).
- [Focused header and title](store-shell-evidence/source-candidate-header.png).
- [Source/candidate narrow layouts and drawer](store-shell-evidence/narrow-before-after.png).
- [All original captures, event counts and actual mock operation counts](store-shell-evidence/results.json).

## Findings, fixes and comparison history

1. **P1, shell not integrated.** The earlier implementation retained a truncated small logo, dark active navigation, an external authentication strip and a duplicate enterprise card above the main title. This was existing markup/theme ownership, not a missing stylesheet. The 240px shell, original 52px brand, 74px header, 40px Store title, blue active rail, sidebar enterprise selection and account placement now follow v4. Focused and full comparisons were opened after the change.
2. **P2, desktop filters wrapped into three rows.** Shared full-width inputs/selects overrode the intended flex layout. The separate local filter fix bounds only Store flex items. At 1200/1280/1440px the controls occupy one row; 44px heights follow the source. The table fills its desktop container and has a 700px scroll surface at narrow widths. No filtering or API handler was rewritten.
3. **P2, narrow shell consumed the content column.** The old 240px sidebar left approximately 110px for content. The revised 390px view has a full-width content column and explicit menu/close controls. Background content becomes inert while the drawer is open. Store table actions can be focused and brought into view by the existing horizontal table scroll container.
4. **P2, complete keyboard traversal exposed a closed-details edge case.** An intermediate uncommitted browser run escaped the drawer after its account summary: Chromium reported rectangles for absolute descendants of a closed `details`. Reverse Tab tried to focus hidden controls. The final focus list explicitly excludes those descendants. Focus return is queued until the parent clears background inert and is invalidated by a newer opening. The same complete forward/reverse traversal now remains inside; Escape closes and returns to the menu button. A component regression includes the nonzero-rectangle closed child.
5. **P2, search/quick-launch shortcut ownership.** Full BusinessShell inspection found its existing Ctrl K handler would also open quick launch after the new search handler. Store search claims plain Ctrl/Cmd K in capture phase; the existing launcher respects a claimed event. Ctrl/Cmd Shift K still reaches the original launcher everywhere, and plain Ctrl/Cmd K remains available outside the Store list. Both real components were exercised together; no instance was started. The expanded tools group and launcher show the accurate shortcut.
6. **Post-fix iteration.** Actual browser checks also verified account Escape/focus return, existing Store confirmation and audit/receipt modal focus, empty-target disabled state, missing-kernel text, explicit deep links, selected/required enterprise and denied context. No actionable P0/P1/P2 remains in the bounded shell/filter/navigation scope.

## Fidelity surfaces

| Surface | Verified result |
| --- | --- |
| Fonts/type | Source font/fallback stack reused; 28px brand, 40px desktop title, 16px navigation and existing readable helper hierarchy. Narrow title is 28px. No external font download. |
| Spacing/layout | 240px sidebar, 52px logo, 74px header, 30px/32px main insets, 48px nav rows, 49px enterprise control. Desktop filters share a row; 390px content and keyboard-reachable row action do not overflow the page. |
| Colors/tokens | V4 `#1b2b47`, `#56657a`, `#1259e4`, `#e9f1ff`, `#f3f6fb`, `#d2dce9` are scoped to the signed-in light shell. Non-light production palettes keep their own tokens. Existing Store disabled/portal styles remain. |
| Images/icons | Supplied brand bytes reused; existing Lucide library retained for standard UI icons. No invented platform art, account avatar image, team role or readiness badge. |
| Copy/content | Actual enterprise, authenticated subject, catalog fields and existing native permission controls remain authoritative. No demo commerce/team/procurement entries are fabricated. All former navigation destinations remain accessible through explicit groups, help/notification links and the account panel. |

## Behavioral validation

- Combined PR24 → 26 → 28 → 29 → 30 plus independent PR27: **246/246 frontend tests**, 19 files; TypeScript/clean Vite production build pass.
- Eight new real-component regressions cover portal placement/read scope, every previous navigation destination, search/query preservation, disabled editor search and dirty cancel, enterprise duplicate/CAS/generation, original API-grant/logout handlers, drawer focus/inert and actual BusinessShell shortcut ownership.
- **21 browser captures** pass with zero runtime exceptions, console errors, unexpected mock methods and external HTTP request attempts. Actual mock operation counters distinguish underlying calls from repeated traversal through the existing logger wrappers.
- No save/open/close/check/prepare/create/start/login/logout/API-grant operation occurs in browser captures. API-grant/logout and enterprise selection tests invoke synthetic native functions only. They are not real-account evidence.
- The combination previously passed the full offline Go race suite: 1719 test events / 555 top-level tests, including all 14 CI critical tests; vet and offline launcher/distribution/Shell-link guards pass. This UI increment changes no Go source or guard script. Final Windows compilation embeds the new frontend without executing it.

## Intentional boundaries and remaining gates

- Source `商品与任务` and `团队协作` have no implemented production destination in this slice. Existing automation/bookmark/tag/log/document/notification entries remain under visible expandable tools; kernels/extensions and settings/backup retain their real pages. No business entry is deleted to resemble a demo.
- Current Store filtering/columns still represent the real authorized catalog, not the prototype's simulated runtime/network inventory. The broader list/detail composition and other business pages are separate visual scope.
- The existing receipt body still exposes its read-only DTO JSON. This increment preserves its behavior and theme; customer-oriented receipt presentation is not claimed complete.
- No official-core serving, real proxy/target, customer identity/Store/CDN lifecycle, installed candidate or integrated installer acceptance. The publisher target catalog remains empty. Prior Xray test-mode results do not prove forwarding. Mihomo executable provenance/config acceptance remains unverified.
- No merge, Ready transition, deployment, release, account/network change or public write. A new public shell PR requires specific publication approval; local implementation and evidence are complete for review.

Bounded shell/filter/navigation final result: passed

Overall product visual/native release approval is not granted by this report.
