# Issue #3: negative integration coverage

This batch starts from main 8eb05f6a9d1070e20ae44e32061efed14cef7b15 and supplements the existing authentication tests. It does not replace their prior evidence or the separately approved native run.

| Boundary | Production entry and observable result | Evidence |
|---|---|---|
| Wails facade after logout/deadline expiry | Actual BrowserProfileCreate/Delete/List reject; real SQLite rows and manager catalog remain unchanged | TestAuthBoundRealStorageRejectsRevokedIngress |
| Authorized normal delegation and replacement | Same App/DAO accepts real Wails and HTTP profile-only creates; old generation and inherited API grant reject; explicit new grant restores only new calls | Same test |
| Full HTTP identity/key pipeline | Existing LaunchServer.Start on test loopback:0, including localhost middleware; no NewTestHandler shortcut or business starter mock | Same fixture |
| Body admitted under an old generation | HTTP 100-Continue marks the production decoder's first read; logout/replacement/new grant precede body release; old write returns 401 with zero rows; fresh write returns 201 | TestAuthBoundHTTPBodyCannotAdoptReplacement |
| CDP proxy after logout/deadline expiry | Real production WebSocket upgrade, then both proxy legs close; new HTTP/CDP upgrade gets 401, zero browser command frames, protocol endpoint remains alive | TestAuthBoundCDPExpiryLogoutAndReplacementCloseOnlyProxy |
| CDP replacement already granted | Old connection still closes despite a valid new identity/API grant; only a fresh connection can reach the protocol endpoint | Same test, replacement-granted |

The sole application-interface substitute is the existing private mockSession. App, typed facade, scopedHTTP factory, profile manager, SQLite DAO, production Start/router/middleware and reverse proxy are real. CDP's external peer is a local protocol fixture; no Chromium/kernel is run. All storage is under t.TempDir; all listeners are test-owned ephemeral loopback and close in cleanup. There is no exported principal setter, fake production login or new product bypass.

The CDP fixture signals each completed upstream upgrade through a channel before connection assertions or revocation. The checks do not assume the server goroutine has updated its counter merely because the client received its handshake.

## Regression and minimal fix

The first new integration run exposed HTTP 500 for a profile write whose captured identity was revoked during body decoding. The complete red case verifies the real catalog and SQLite stay at zero, but receives 500. mapProfileWriteErrorStatus treated the typed errRequestRevoked as an unknown server failure. The fix maps errors.Is(err, errRequestRevoked) to 401 before ordinary write-error mapping. It changes neither admission timing, committed-write receipts nor unrelated error classes.

Red exit 1 and green receipts are retained locally at build/dist/verification/issue3-negative-ingress-20261025/. Final full offline Go race: 397 top-level / 1235 including subtests, zero failures/test-level skips, stderr zero. Frontend 41/41 remains complete; TypeScript/build, vet, compile and exact-HEAD CI results are recorded with the PR and local receipts. Generated EXEs are compile-only, NOT_RUN.

The CI's existing critical Package+Test guard now also requires these three integration tests, without filtering the full Go suite, skipping failures, upgrading Actions or changing permissions.

Local TypeScript/clean build, full vet and Windows desktop,production compile also passed, exit 0. Go/vet/native stderr are zero bytes. The compile-only EXE is 52,172,288 bytes, SHA256 7623cb248b16eac9c3b424dbaa86a267a8e421e33f6631172df270203abaea73, NOT_RUN. The existing nonblocking stale Browserslist-data warning is retained; dependencies were not changed.

## Existing coverage reused

- Actual main.desktopBindings inventory and unconfigured denial: TestProductionDesktopBindingsHaveCompleteAdmissionPolicy.
- All facade method policies and stale scopes: TestAuthBoundRejectsEveryBusinessMethodBeforeDelegation and TestAuthBoundDelegatesOriginalAndPreservesSharedState.
- Native dialog/preview, queued backup and maintenance-lock revocation: existing auth_dispatch_test.go cases.
- Async/sync installation revocation cleanup: existing runtime_manager_admission_test.go red/green cases.
- PKCE, signature/claims, wrong state/replay/cancel/expiry and logout versus in-flight exchange: existing desktopauth mock-TLS tests.
- Explicit API grants and generations: TestAPIGrantIsExplicitAndNeverAdoptsNextSession.
- App-only/tray retention and one ownership-proven normal Browser.close: existing auth_cleanup_test.go cases.

## Proof boundary

This is offline integration evidence, not an additional real EXE/Wails/IdP acceptance run. The prior 23b/d4988f native normal-flow PASS remains tied to that package, including human-assisted logout and its tool-correction history.

Still NOT_RUN under this batch: direct negative probes against a newly built native EXE after logout/real expiry, real account switching/replay, and extended Windows OS ownership teardown with a real browser. They require a concrete artifact-specific runtime decision. No new real login, IAM/secret read, retained Docker/service action, kernel, deployment or security setting change occurs here. The normal-flow screenshots and trial/profile data are not touched.

Further authorized offline gaps should be identified against this matrix and existing behavior tests; do not duplicate tests or grow a replacement control framework.
