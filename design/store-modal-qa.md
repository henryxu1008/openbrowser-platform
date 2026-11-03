# Store portal theme slice

Related to #21. Stacked on PR24 at `b489877839415dfd7a53ee64d2c0d0dfa6c50201`; this report covers the portal increment only.

## Source and comparison boundary

The approved option 1 v4 HTML (2026-10-31 palette correction) was materialized through Library and verified: 521087 bytes, SHA256 `3a61a3c2386d264dabe7401835d4f6784fd22c73dc0f37db71315cf865a56efd`. The source ZIP was also read, SHA256 `ef6d2b51dcc7c6b1c7bf5e5bd7a2cac65ff42658c08504c42a16b31f3e1d1680`.

The original HTML was rendered in an isolated browser tab. Its actual workspace dialog provides the shell reference: [source capture](store-modal-evidence/v4-workspace-light.jpg). It does not contain the production unsaved-enterprise, audit, receipt or CAS-error states. Accordingly, these are comparisons of the common shell and palette, not identical business states or a claim of pixel equality. Existing production copy, warnings, widths (400/420/700/760px), close/confirm behavior and semantic data stay in place. No prototype business timers, procurement, payment or network-readiness claims were copied.

Source and wide candidate: CSS viewport 1280 × 900, DPR 1, screenshots 1280 × 900 pixels. Narrow candidate: CSS viewport 390 × 844, DPR 1, screenshots 390 × 844 pixels. No density resampling. Comparison boards add a 40px label strip; focused boards crop the original pixels without scaling.

- [Full source/candidate comparison](store-modal-evidence/source-candidate-full.png)
- [Focused source/candidate comparison](store-modal-evidence/source-candidate-focused.png)
- [Same-state before/after confirmation](store-modal-evidence/before-after-focused.png)
- [Same-state narrow light/dark detail](store-modal-evidence/narrow-light-dark.png)

All four boards were opened and inspected together with the individual rendered states. Screenshots contain prototype or synthetic fixture data only.

## Findings and fixes

1. **P1, fixed:** body portals missed the Store light palette. The baseline rendered global text `#1e293b`, muted close control `#94a3b8`, no surface border and danger `#ef4444`. Explicit portal opt-in now supplies v4 Store tokens while other dialogs retain their defaults.
2. **P2, fixed:** shell rhythm and neutral button boundary differed from the rendered source. Final measurements match the source shell: 23px/32.2px bold heading, 15px body with 27px paragraph line height, header 25px 28px, body 26px 28px, footer 20px 28px, 16px radius, 44px footer controls and a 1px light surface border `#d2dce9`. Neutral light action border is `#b6c4d7`; the scoped rule also overcomes the existing global button border reset.
3. **Narrow adaptation, verified:** at 390px the definition list uses one column, its body scrolls vertically, the close control stays visible, and both record/receipt `scrollWidth` equal their client widths. Receipt JSON wraps. No baseline narrow-record failure is claimed.

Five new scope regressions failed against the base and pass after the change. The shared Modal change is an optional root class plus inert part classes; its state, timers, portal target and callbacks are unchanged. Store handlers receive only the opt-in class. The standalone editor is not opted in.

## Required fidelity surfaces

- **Typography:** the portal uses the source font stack and measured heading/body rhythm. Existing compact record and JSON typography remains readable. No font download was added.
- **Spacing/layout:** the measured shell spacing/radius is mapped above. Existing production widths are intentional; the longer enterprise title wraps. Narrow controls remain reachable.
- **Colors/tokens:** light computed text `#1b2b47`, muted `#56657a`, accent `#1259e4`, white surface and `#d2dce9` border match v4. Confirming controls were measured in the real DOM: text `#56657a`, fill `#e5ebf3`, border `#c1cddd`, opacity 1. Production dark palette remains `#27272a` / `#fafafa` and uses the same geometry; its colors are not replaced by prototype dark colors. Cream/mint/ocean keep their existing tokens and light native scheme.
- **Assets:** existing production icons are retained. No custom drawing, generated imagery or prototype brand asset was introduced into product code.
- **Copy/content:** existing production confirmations, evidence fields, error and receipt text are preserved. The prototype has no exact equivalent copy for these states; exact same-state visual fidelity is NOT_RUN.

## Interaction and offline verification

The fixture is [store-portals.html](../frontend/test/fixtures/store-portals.html), run through the repository's existing Vite server. It imports real Store pages, BrowserEditPage, Modal/ConfirmModal, adapters, auth capture and generated bridge. Only native methods/events are synthetic. Unexpected native methods and fetch are rejected. It is a test entry, excluded from the production build.

- Real browser: Tab and Shift+Tab cycle in the leave confirmation; Escape and cancel restore the return trigger and preserve the unsaved name. After each completed exit animation, reopen works.
- Audit detail: single-close-control Tab cycle; Escape restores **查看记录**. Receipt Escape restores **只读核对原操作**.
- Repeated enterprise confirmation: double-click calls the mock selection once with the original revision/auth generation; no save/open/close/prepare call occurs.
- Read-only record/receipt calls traverse the real adapters and mocked generated bridge. The managed CAS-error popup also receives the Store scope, with inputs retained.
- An unscoped control dialog still renders global `#1e293b`, 18px heading and disabled opacity 0.5. No scoped style leaks were found.
- Source and final fixture console error logs: empty. Expected Vite/React Router advisory warnings are not production failures.
- Full frontend: **156/156** tests. TypeScript and clean Vite build pass. Mocked Windows launcher guard: **44 cases pass, product executions 0**. Whitespace check passes.

Representative captures: [leave wide](store-modal-evidence/leave-light-wide.jpg), [leave narrow](store-modal-evidence/leave-light-narrow.jpg), [leave dark](store-modal-evidence/leave-dark-narrow.jpg), [enterprise wide](store-modal-evidence/enterprise-light-wide.jpg), [enterprise dark](store-modal-evidence/enterprise-dark-narrow.jpg), [record wide](store-modal-evidence/record-light-wide.jpg), [record narrow](store-modal-evidence/record-light-narrow.jpg), [receipt wide](store-modal-evidence/receipt-light-wide.jpg), [receipt narrow](store-modal-evidence/receipt-light-narrow.jpg), [save error](store-modal-evidence/save-error-light-wide.jpg), [unscoped control](store-modal-evidence/unscoped-control-light.jpg).

## Remaining gate

The bounded rendered scope, palette, keyboard and mock-bridge checks above passed. Full Store workbench visual approval and exact prototype-equivalent production-state comparison remain incomplete; the approved source does not supply those state counterparts. This report does not close #21 or approve PR24. Native Windows/customer/kernel/CDN lifecycle acceptance is **NOT_RUN**, owned by a separate acceptance task. No running client, real account, proxy, service configuration, kernel, deployment, merge or release was operated.

final result: blocked
