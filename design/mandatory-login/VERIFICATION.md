# Issue #3: offline implementation and verification

The earlier sections retain their dated offline snapshots. The 2026-10-25
install-cleanup follow-up and the scope of reused live evidence are recorded at
the end of this document and in PR #4 / Issue #3.

The implementation is on `codex/mandatory-login`, based on accepted busy-fix
`2e75ad2e5c3b8146ad8e97e9d6136fd50dec2442` (Draft PR #2). This is an offline
code/build batch, not real OIDC or complete native product acceptance.

The original managers, database paths, browser launch/stop and business pages
remain in use. The bound facade adds typed generation checks. `App` and HTTP
request receivers share pointers to original state; mutexes/once values are not
copied. Auth controls do not wait for business I/O or native dialogs. Startup
business work waits for login and retains an initialization failure without
automatically retrying it. Logout revokes new work, not running browsers.

The generated binding policy contains 193 protected methods and seven explicit
auth/cleanup methods. `GetInterceptor` is not bound. HTTP/CDP requires both the
configured API key and a grant from the current login generation. A safe health
response is public. Deferred/background work and dialog completions retain their
initiating admission. The original native tray exit also checks the Go session.

Normal cleanup requires this process's admitted start receipt, matching current
monitor/command, PID creation time, directory and loopback endpoint proof. It
sends only `Browser.close`, once; missing proof or an unknown acknowledgement
has no force/retry fallback. OS proof is implemented for Windows. Other systems
fail closed and retain manual browser-window closure and app-only exit.

Verification completed on 2026-10-24 (Asia/Singapore):

- Frontend: 34 tests PASS, including the existing 22 busy regressions, session
  generation/expiry/late results, all 200 real SDK arities, Record arguments,
  dynamic module loading, login/exit shell and late log rejection.
- Race-enabled production binding/backend/browser/automation regression: four
  packages PASS, 35 top-level tests and 816 tests/subtests. It exercises the real
  binding object, original ZIP import, native-dialog seam, queued background and
  scheduled backup dispatch, deferred tabs, proxy batches, automation admission,
  app-only/tray exit and normal cleanup protocol. Only private mock auth, mock
  OS proof and in-process test HTTP/WebSocket peers are used.
- Race-enabled `internal/desktopauth` and `backend/internal/launchcode` suites
  PASS: mock TLS OIDC/PKCE, claims/signature/strict JSON, expiry/logout races,
  explicit API grants, actual router rejection, late bodies and CDP revocation.
- `go vet` across main and the five relevant backend/auth packages: PASS.
- TypeScript + clean Vite build: PASS. Existing Browserslist data-age warning is
  retained; no dependency/security configuration was changed to suppress it.
- Windows `desktop,production` native compilation: PASS, using cached modules
  with `GOPROXY=off`, `GOSUMDB=off`, `GOTOOLCHAIN=local`, `-mod=readonly`.

Reproduce the generated boundary from the repository root:

```powershell
go run ./scripts/generate-auth
node frontend/scripts/bootstrap-auth-sdk.mjs
```

Normal Wails SDK generation remains compatible with the same facade. The offline
bootstrap uses the existing TypeScript parser; it never splits generic types on
commas. Boundary tests reject policy/signature drift and exposed internal getters.

Limits: no real IdP contact, login, credential generation, product GUI/kernel or
product service startup. Mock listeners are test-only. The compiled EXE was not
executed. Public issuer/Native-client and callback/TLS metadata remain pending.
Local profiles are shared native data; this is not per-account data isolation or
a lock on an already running Chromium window/direct CDP port. Main/PR GitHub CI
is NOT_RUN: current upstream workflows are tag/manual only, with no check/run on
the verified busy head. Do not treat these local results as remote CI.

Raw logs, exit receipts and the compiled EXE are retained locally under ignored
`build/dist/` paths. Existing trial data, frozen busy EXE/ZIP and source backups
remain separate and are not uploaded with this source change.

## Bounded follow-up review

The follow-up stayed within the original PR's 95 files and found two concrete
regressions against `83e55fb`:

- `BackupImportPackage` and `BackupRestoreLocalPackage` return a profile preview
  through the shared producer, but only the instance-list facade had registered
  its ownership. A fresh logged-in confirmation therefore failed with
  `authentication_required`. Ownership now belongs to the shared preview
  producer and the immutable initiating generation. Both backup routes can
  confirm; an old backup preview still cannot cross login generations.
- An import admitted before waiting for `maintenanceMu` could commit after its
  session was revoked; the failing counterexample created one profile. The
  existing maintenance-lock sites in the reviewed App files now recheck their
  original scoped admission immediately after acquiring the lock. Existing
  mutexes and business code remain in place. A revoked waiter creates nothing
  and a fresh caller can acquire the released lock and use the original import.

No live identity/provider, GUI/kernel or product-service run is part of this
review. It does not certify unrelated domain locks or complete product behavior.

The follow-up's applicable four-package race run passes 37 top-level tests and
820 tests/subtests; vet and Windows production compilation pass. Frontend source
and SDK bytes did not change, so the prior 34 frontend tests/clean build remain
applicable. The new EXE SHA256 is
`6910b46fc9c46c5ff7ab414d700d77e0ea698c691a8cc2819e5a4cc5dae5fc90`;
it was not executed. The first `83e55fb` binary and previous frozen busy artifacts
remain retained separately.

## Install-cleanup P2 follow-up (2026-10-25)

Final static review of `4bdef57` found an install-state leak: `InstallAsync` set
`installing=true`, but revocation between the worker's admission check and the
callee's check returned before the callee registered its cleanup. A later login
retained the busy state. The async worker now owns unconditional deferred cleanup;
the callee only clears the flag for synchronous callers that acquire it there.
Each owner releases its flag once, including early admission failure.

The deterministic channel-gated regression invokes the real `InstallAsync` and
revokes the captured generation at both worker and callee checks. It verifies
cleanup, admission of a new generation, no progress events and no runtime files.
Bundled-node mode with an invalid version prevents runtime download or process
dispatch even if an unexpected admission occurs. A separate synchronous failure
regression preserves the synchronous owner's cleanup.

The callee-gap regression fails on the original code (exit 1, retained installing
at check 3). With the patch, the applicable four-package race run passes 39
top-level tests / 824 tests and subtests (exit 0); vet and offline Windows
production compilation pass. No unchanged frontend or OIDC/HTTP suite was rerun.
The new compiled EXE SHA256 is
`416fa958fe5c9f571d63b774835f42d9eb377cba5d6541b3a9b698961ae0ce19`;
it was not executed. Red/green logs, stderr and exit receipts are retained under
`build/dist/verification/issue3-install-cleanup-20261025/` and are not committed.

The prior approved normal-login acceptance remains evidence for fixed head
`4bdef570432e504c3999eaab6b30233f53139087` and EXE SHA256
`6910b46fc9c46c5ff7ab414d700d77e0ea698c691a8cc2819e5a4cc5dae5fc90`:
user-completed login, original-page unlock, logout UI blocking and normal app-only
exit 0. All 21 evidence-file hashes and the original/trial EXE hashes still match.
That live run had stdout 1,035 bytes and stderr 64 bytes; its bounded limits and
listener-attribution correction in Issue #3 remain. It is not a live run of this
patch, nor proof of the install-revocation path, which is covered offline here.

The reviewed P2 is resolved in code and regression. No additional merge-level
code finding was identified in this bounded follow-up. PR #2 remains unmerged,
both PRs remain Draft, and PR #4 still targets the dependency branch. Resolve the
dependency/base/commit range under separate merge authority. Exact-head remote
CI is tracked separately from local checks and remains NOT_RUN when no check is
present. No product/kernel/service run, IAM change, merge or deployment occurred
in this follow-up; old binaries, data and evidence were preserved.

## Follow-up: latest fixed EXE FAIL and safe login diagnostics (2026-10-25)

The preceding P2 and dependency/base statements are historical snapshots. PR #2
was merged at f753a4909b93f4c5fadb4ac28b4e02632ce1f4b7 and PR #4 retargeted to main,
without changing its four-commit authentication patch/tree at that time.

A newly approved single start of c26571a / EXE SHA256
416fa958fe5c9f571d63b774835f42d9eb377cba5d6541b3a9b698961ae0ce19 failed after one
native browser-login click: the gate displayed the generic session-expired text
and the original business page never unlocked. Browser dispatch and OIDC request
occurrence remain UNKNOWN; raw native Status was not captured. No login retry,
identity step, logout, kernel/proxy start or additional service was performed.
One approved app-only quit completed with actual exit 0, stdout 106 bytes and
stderr 64 bytes (only WebView2 environment-created information). Later exact
window/process/listener observations were empty. This FAIL blocks #4 and #6;
old 4bdef57/6910b... PASS and CI success cannot replace this result.

Bounded source review established a diagnostic defect, not the live failure's
specific cause: discovery request, JSON and metadata failures were collapsed
into identity_login_failed, and the gate called every errorCode an expired
session. The production Login entrypoint is still generation-checked before
request/browser dispatch; no business gate, SDK shape, TLS or claim check was
weakened. The browser opener still has no success acknowledgment, so opening a
browser is never inferred from its void call.

Tests were added before the fix. Four in-memory transport cases failed red
(exit 1) because their safe stage was lost; stale-generation rejection already
passed without request/browser dispatch. Four actual-generated-SDK gate cases
failed red (exit 1) because they displayed expiry instead of the failure stage.
The fix preserves fixed local request/document/metadata/setup codes in Status
and logs only those codes outside the identity mutex. It never logs provider
bodies, URLs, transport errors, authorization codes, tokens or identity data.
The gate displays known safe descriptions, keeps true expiry separate and uses
a fixed fallback for unknown codes. There is no replay or new login mechanism.

Validation after the fix: complete frontend 41/41 PASS, including actual SDK
single-call/no-replay and a late pre-login poll that cannot unlock business;
TypeScript and clean Vite build PASS. desktopauth + launchcode race passed 16
top-level / 38 including subtests. Moving logging outside the identity mutex
was followed by a final desktopauth race run: 11 top-level / 29 including
subtests PASS; unchanged launchcode remains 5/9 PASS. All passed command exits
are 0. The frontend build retains a nonblocking stale Browserslist-data warning;
no dependency update was performed. Red/green raw logs and exit receipts are
local under build/dist/verification/issue3-login-diagnostic-20261025/ and the
sibling issue3-login-diagnostic-frontend-20261025/ directory.

The failed run's raw streams, screenshots, exit/ownership receipts and existing
frozen EXEs remain unchanged. The exact original live failure remains UNKNOWN;
this patch improves safe diagnosis and corrects misleading UI, and does not
claim to repair or pass that real login. Any newly compiled EXE remains NOT_RUN
without fresh artifact-specific runtime authority. No real IdP request, Cookie/
token/account read, IAM/config change, product retry, merge or deployment occurred
in this offline follow-up.

Applicable two-package vet and Windows desktop,production compile also passed,
exit 0, stderr 0 bytes. The new compile-only EXE is 52,165,120 bytes, SHA256
d4988f695a4bfd49d3f3f64b9cd258950f896d1290e36d615f8cfdd32c957785, NOT_RUN.
