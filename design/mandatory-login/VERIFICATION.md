# Issue #3: offline implementation and verification

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
