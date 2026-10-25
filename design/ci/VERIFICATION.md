# PR and main CI

Issue [#5](https://github.com/qq550723504/openbrowser-platform/issues/5) adds
`.github/workflows/ci.yml`. It runs on PR events and ordinary pushes to `main`
or `codex/**`, including Draft PRs. It does not publish or deploy artifacts.

This change is stacked on PR #4, fixed base
`c26571a72ee5352d49c99f5eaa6b172a44fc2ad1`. Its PR targets
`codex/mandatory-login`, not main: the auth code remains owned by #4. Do not
retarget the CI PR to main before that dependency is resolved. Neither PR is
merged by this batch. Existing main/#4 NOT_RUN snapshots stay historical until
this workflow reaches those branches; new CI runs are identified by exact head.

## Coverage

A Windows 2022 runner uses Node 24.18.1 and Go 1.26.5, matching the known local
toolchain. It reuses the official checkout/setup actions, package-lock.json,
go.sum and frontend scripts. The runner's existing GCC supports CGO/race builds;
no extra compiler, Wails CLI or browser installer is added.

The initial a49e875 push/PR runs passed but reported the old action runtime being
forced from Node 20 to Node 24. The new CI workflow uses the supported v6
checkout/setup actions instead. The existing publishing workflows are untouched;
the follow-up head must finish its own push/PR checks before closure.

1. `npm ci --no-audit --no-fund` installs locked frontend dependencies.
2. `npm test -- --reporter=dot` runs the complete existing Vitest suite.
3. `npm run build:clean` runs TypeScript and builds clean Vite assets for Go embed.
4. `go mod download` and `go mod verify` prepare checksum-verified Go modules.
5. `go test -race -mod=readonly ./... -count=1 -timeout=5m -json` runs the full
   Go unit suite with GOPROXY/GOSUMDB off after module preparation. There is no
   test-name filter. Nine exact Package + Test behavior pairs must explicitly pass,
   including native binding, backup previews/maintenance revocation, async
   install revocation, PKCE and API/CDP revocation. Empty selection or a same-name
   test from another package cannot pass this guard.
6. `go vet -mod=readonly ./...` and Windows `desktop,production` compilation
   run offline. The resulting EXE is not executed or uploaded.
7. Tracked-source drift fails the job.

The full-suite baseline at c26571a exposed two old test contract mismatches:
the maintenance cancellation fixture attempted dispatch without a session, and
the quit-state test expected implicit full shutdown in the new authenticated
host. The test-only adaptation reuses the private mock session and asserts the
current host-preserves-runtime contract while retaining the legacy core,
app-only and explicit full-quit assertions. No product implementation or
authentication bypass is added. Baseline failures remain in local evidence.

Local preflight after these two test-only adaptations passes all 34 frontend
tests, TypeScript/clean Vite build, and the unfiltered Go race suite: 392
top-level tests / 1,221 including subtests, with no skipped tests. The exact
workflow Package + Test guard also accepts that JSON result. Full Go vet and
Windows production compilation pass; the executable is not run.

Local all-module verification could not complete in offline mode because an
unused Windows dependency-graph entry (go-iptables v0.8.0) was absent from cache.
That failure is preserved rather than labeled PASS. CI prepares the full locked
module set using normal Go downloads, then verifies checksums and runs the
behavior/vet/build stages offline. Its actual result must be checked separately.

## Boundaries

Workflow permissions are `contents: read`, checkout credentials are not
persisted, and there is no `pull_request_target`, production credential, secret
input, release step or artifact upload. Tests use temporary directories and
in-process fixture listeners. They do not start the desktop GUI, browser/proxy
kernels, real OIDC login or any retained product service. No IAM, main protection
or required-check setting is changed.

CI behavior/build evidence is separate from product acceptance. The prior real
normal-login PASS belongs to 4bdef57 / EXE 6910b46f...fc90. The c26571a EXE
416fa958...0ce19 remains unexecuted; install revocation is demonstrated by offline
tests. A CI pass does not relabel either binary's live acceptance.

## References

- [Runner's Windows 2022 software inventory](https://github.com/actions/runner-images/blob/main/images/windows/Windows2022-Readme.md)
- [Go setup and module/build caching](https://github.com/actions/setup-go/tree/v6)
- [GitHub workflow trigger semantics](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/trigger-a-workflow)

Local logs and exact source/run receipts stay in the ignored
`build/dist/verification/issue5-ci-20261025/` folder. GitHub run/job links and
terminal results are recorded in Issue #5 and the linked Draft PR.
