# Default Store entry: bounded verification

Related to #21. A signed-in fresh `/` previously opened the standalone instance list and its **新建配置** action. The approved v4 prototype starts with the Store workbench. The only production change is the root `Navigate` target, from `/browser/list` to `/stores`, retaining `replace`.

## Base and relationship to PR24/26

The smallest code base is main `10ba90eeb08ab941bc4eb2add57da8e18d1970a6`: it already has the Store route, enterprise selection, authorization and catalog. `AppRoutes.tsx` is identical on that main commit and reviewed PR26 `b2ba5d566c02417b343194694fc6eaa0a8678b45` before this one-line change. This PR targets main and does not include or modify PR24/26 commits.

There is no code prerequisite on PR24/26 for the default redirect. The approved Store page and portal visual work remains owned by those Draft PRs; this is a companion entry increment, not their visual approval or a release approval. A disposable integration checkout at exact PR26 plus the identical route/test/fixture delta also passed its complete frontend suite and clean TypeScript/Vite build. The reviewed branches were not changed.

## Prototype and rendered evidence

The v4 HTML was read locally and rendered again: 521087 bytes, SHA256 `3a61a3c2386d264dabe7401835d4f6784fd22c73dc0f37db71315cf865a56efd`. Its `src/App.jsx:39` defaults to `stores`. The target here is the default destination and active navigation, not exact table data or complete shell styling.

- [Before/after root destination](default-entry-evidence/before-after-root.png): exact main baseline versus this candidate, both 1280 × 900.
- [v4 versus the PR24/26 integration](default-entry-evidence/v4-combined-destination.png): both enter the Store workbench; production continues to show authorized catalog facts only.
- [No selected enterprise](default-entry-evidence/after-root-select-enterprise.jpg): `/stores`, explicit enterprise selector and selection guidance.
- [Existing enterprise context](default-entry-evidence/after-root-existing-enterprise.jpg): `/stores`, existing server context and catalog reused.
- [Signed out](default-entry-evidence/login-gate.jpg): login gate remains at `/`.
- [Denied context](default-entry-evidence/denied-root.jpg): denial stays visible at `/stores`; no standalone-profile fallback.

The source and candidate images, full comparison boards and relevant navigation regions were opened and inspected. CSS viewport was 1280 × 900 at DPR 1. Candidate captures are 1280 × 900 pixels; the source capture is 1265 × 889. Boards preserve original pixels without resampling and add a 40px caption strip. This supports destination/page-identity verification, not a pixel-equality claim.

Typography, spacing, colors, icons and copy are unchanged by this production delta. Main retains its existing shell; the integration capture inherits PR24/26 Store styling. Their remaining overall visual gates stay separate. No prototype simulated counts, readiness, purchasing, payment or network behavior was imported.

## Offline behavior checks

The new composition cases keep the real AuthGate, AppRoutes, Store pages, lazy standalone page, adapters and generated bridge. Only native calls/events and the renderer portal boundary are mocked.

1. Signed-in root with no enterprise: reaches `/stores`, shows explicit choice, does not select an enterprise or read a catalog automatically.
2. Signed-in root with an existing enterprise: reaches `/stores` and reads that authorized catalog with the captured revision/generation.
3. Signed-out root: business pages stay unmounted; only a verified synthetic sign-in snapshot permits the Store destination.
4. Denied Store context: stays blocked at `/stores`, with no standalone fallback or profile read.
5. Explicit `/browser/list?view=table`: retains the destination and query.
6. Explicit Store record deep link: retains path/query and the real read adapter.
7. Explicit Store editor deep link: retains path/query and the real options adapter.

Every case asserts zero login/logout/grant/quit, enterprise selection, profile creation, environment save/open/close, browser start and kernel preparation calls. Four default-entry cases first failed at `/browser/list` on main; all seven pass after the one-line fix. The three explicit-route cases already passed before the fix.

Real browser fixture checks confirm the default enterprise choice, existing enterprise context, signed-out gate, denied context and explicit standalone destination. Source, candidate and integration console error logs were empty. All displayed data is synthetic. [The fixture](../frontend/test/fixtures/store-entry.html) is a test-only Vite entry and is excluded from the production build.

Validation on the main-based candidate: **123/123 frontend tests**, clean TypeScript/Vite build, whitespace check; mocked launcher **26 cases**, distribution metadata **20 cases**, and isolated Shell-link guard **34 assertions** pass. Product/installer executions: **0**. The PR24/26 disposable integration's full frontend suite and clean build also pass.

## Limits

This increment does not change auth, permission, native admission or any business handler. Live `store-browser` entitlement shortage is not solved by a redirect and was not probed. Real authentication, account, Store, proxy, kernel/CDN and native lifecycle acceptance remain **NOT_RUN**. Overall v4 shell fidelity is not approved here. No running client, service configuration, merge, deployment or release was operated.

Scoped default-destination and boundary checks are complete; independent review and exact-HEAD CI are tracked in the Draft PR.

final result: passed
