# Publisher targets and operation-owned bridge increment

Related to #22. Based on PR29 `d3cb726b20f3ae60759ff6aa96003b9ce90bd4b1`.
This increment closes two native integration gaps within the existing scoped
Store check contract. No external target, running service, customer credential,
shared bridge, live client or network setting is operated.

## Publisher catalog source

The catalog is **release source compiled into the Go binary** via `go:embed` at
`backend/internal/storeproxy/publisher_targets.json`. It has no runtime file,
environment, command-line, API, user settings or renderer override. It does not
derive a target from `apiOrigin`, load an adjacent installed JSON file, fetch a
remote manifest, or send identity tokens. Changing targets requires a reviewed
source change and a new binary under the existing release process; embedding
does not claim a new signing or distribution guarantee.

The shipped source remains exactly an empty schema-v1 catalog:

```json
{"schemaVersion":1,"targets":[]}
```

`PublisherCatalog` bounds the source to 32KiB, validates UTF-8, rejects duplicate
keys (including escaped aliases), extra/trailing values, unsupported schema,
unknown or incorrectly cased fields, missing/null targets, excessive nesting and
decoding work. Every entry requires exact `id`, `label`, `url`, `expectedStatus`
fields and passes existing immutable `NewCatalog` validation: at most eight unique
targets, bounded identifiers/labels/URLs, credential-free HTTPS DNS names on
port443, no query/fragment/IP/local hostnames, and one to four positive statuses.
Native returns fixed catalog errors without raw input. Catalog metadata is copied;
the existing fingerprint and ID must still resolve natively at execution.

`NewApp` consumes only the embedded catalog. Invalid release data fails closed to
an unavailable catalog while leaving the login shell usable. A test requires the
shipped catalog to parse and remain empty; malformed source therefore fails CI.
Installed spoof files and the existing user-editable multi-target settings cannot
replace it. Existing Store, identity, enterprise, permission, binding CAS and
operation cancellation gates remain mandatory before any network capability.

## Owned bridge lifecycle boundary

The browser's existing resolver remains the protocol/stack authority. Standard
credentialed proxies require Xray in the configured Xray stack or Mihomo in the
configured Mihomo stack. The existing no-credential native subset is unchanged;
there is no authenticated native shortcut or cross-stack fallback. This bounded
increment does not add encrypted-protocol parsing or downloads.

An owned bridge must have one operation-owned process handle and private
credential input, bounded startup, operation cancellation, stop and joined exit.
It must not borrow `EnsureBridge`/`AcquireBridge` shared-manager references, kill
by process name/port, accept a caller command line, expose raw process output, or
release the Store/session gate before the process and its I/O finish. The native
adapter is now bound to the scoped use case; the empty catalog prevents activation.

`storebridge.Connector` now derives the engine through the existing resolver,
limits startup to three seconds within the operation context, and owns a lease
whose cancellation stops only its returned process handle. Failed, canceled,
late and panicking readiness paths join process exit and private I/O before
returning. Repeated cleanup is idempotent, including racing cancellation watchers.
Cleanup helpers are additionally fault-contained. Async Stop cannot escape as a
panic; both Stop and Wait are attempted even when either panics. An independent
non-nil `Settled` channel must prove actual exit and all I/O cleanup, so helper
failure never acknowledges settlement. Cleanup faults become a fixed unavailable
result after joining, including when target evidence had otherwise succeeded.
The concrete process worker owns that signal independently of helpers.
Input formatting is redacted; decoded credentials travel on the child's
stdin, never argv, environment or files. Process errors/output are not returned.

`NetworkProber` has a narrow native-only `Connector` port. It permits only a
credential-free `127.0.0.1` SOCKS5 endpoint from the owned adapter, repeats authority
checks after startup, and joins transport cleanup before bridge cleanup. Partial
bridge startup cannot reach a target or fall back to direct. The production
prober now constructs the real Starter from an immutable native snapshot of the
configured stack and existing runtime paths. These fields are never request data.
The existing general-ingress fence is held exclusively through cleanup, including
for an unsaved Store, to prevent generic configuration writers from changing that
snapshot. Concurrent generic calls wait until cleanup; the original Store guard
then decides their admission. Store/identity/enterprise/binding authority is still
rechecked at stages and before publishing evidence.

## Concrete Windows Starter and engine configuration

The candidate supports Windows ownership verification. Other platforms keep
credentialed checks closed until an equivalent OS proof is implemented. The
existing no-credential native path is unchanged on those platforms.

`ExecStarter` uses the already native-configured Xray/Mihomo executable path. With
no explicit Xray path it uses the installed `bin/xray.exe`, requiring the existing
release owner's embedded `publish/runtime-manifest.json` digest. Missing explicit
Mihomo configuration fails closed. Explicit native paths retain the browser's
existing native-configuration trust authority; this does not claim publisher
signing/provenance for custom executables. No path, stack, flags or command can be
supplied in the Store request. There is no PATH/environment/other-client search,
download, installation or automatic stack fallback.

Paths are local, regular, bounded-size files with reparse ancestors rejected. A
Windows read-share-only handle pins the selected file through process exit and
I/O cleanup; readiness checks its file identity again. Each operation has a private
temporary home. Only that generated home is eligible for removal. No proxy
configuration/credential file is created, and existing client workdirs are unused.

The real argument forms are `xray run -format=json -config stdin:` and
`mihomo -d <owned-home> -f -`. Source evidence:
[Xray's run command](https://github.com/XTLS/Xray-core/blob/v26.2.6/main/run.go),
[Xray stdin documentation](https://xtls.github.io/en/document/command.html), and
[pinned Mihomo stdin handling](https://github.com/MetaCubeX/mihomo/blob/v1.19.20/main.go).
`BuildOwnedStandardProbeConfig` extracts the existing standard proxy mappings
without managers/loggers/network TLS-pin fetching: one loopback SOCKS listener,
one upstream and an explicit rule to it; no direct outbound, UDP, providers,
controller, external UI, DNS listener, cache persistence or geo auto-updates.
HTTPS proxy TLS verification remains enabled. Configuration is bounded JSON sent
through stdin followed by EOF, then its serialized buffer is cleared. The earlier
generic fixture envelope now exists only in tests, not production code.

The child environment has only a small OS/locale/temp whitelist; proxy, identity,
CLASH/XRAY config and other caller overrides are absent. stdout/stderr go to the
OS null device. Readiness verifies the exact owned PID's loopback listener via
Windows TCP owner metadata, performs only a bounded SOCKS greeting (no CONNECT),
then verifies ownership/file identity again. An unrelated process on the same port
cannot satisfy readiness. Existing transport bounds then cover the one target.

The process owns an independent operation-deadline watcher. Cancellation kills the
exact handle **before closing stdin**: Windows anonymous-pipe Close can otherwise
wait behind a blocked writer and prevent termination. One worker waits actual
exit, joins stdin and cancellation callbacks, closes runtime/null handles, removes
the private home and only then closes `Settled`. Stop/Wait return fixed cleanup
errors; the lease also contains their panics and never substitutes a helper return
for that independent proof. Transport joins before bridge cleanup and native
Store/session gates remain held until both complete.

## Target status

The previously investigated UI health candidate has been withdrawn. Source-level
routing and ingress declarations are not proof of a currently deployed service.
Do not configure, contact or restore that candidate. There is no approved active
endpoint; the embedded catalog stays empty. Development and local process/config
fixtures do not require a new service. A minimal target contract and any deployment
or real-network acceptance authorization will be resolved only when end-to-end
acceptance actually needs them. No third-party target is selected as a substitute.

## Evidence and release status

The source loader's tests exercise malformed/ambiguous manifests, copied metadata,
version changes, safe empty defaults and attempted installed/user-settings
injection. No runtime health endpoint has been probed. Real proxy, region,
merchant-login and customer acceptance remain NOT_RUN. Keep Draft for independent
review; no merge or deployment.

Lifecycle fixtures execute this test binary with fixed arguments; the real Starter
fixtures build a fixed Go core substitute. It binds only loopback, checks the real
argument/config shape and answers SOCKS greetings. It never dials, resolves a host
or forwards CONNECT. Native end-to-end fixtures exercise both configured stacks
through the real scoped facade, actual Starter/PID readiness and a mock target
transport, with zero save/open/admit/report effects. Config fixtures check existing
mapping semantics and exact decoded synthetic credentials; no official engine
parser/process is executed or new dependency installed.

Tests cover blocked stdin, process exit, independent lifetime cleanup, unknown or
unpinned runtimes, cross-stack rejection, invalid readiness, late handles, readiness
and Stop/Wait panics, cleanup-before-settlement, and native settings serialization
before and after a Store binding exists. The Wait-panic regression failed on the
prior candidate with premature settlement; the new tests pass. Key lifecycle and
blocked-stdin regressions passed twenty race-enabled repetitions. The complete
offline Go race suite passed **1719 test events including subtests / 555 top-level
tests**, plus full vet and Windows
production compilation without product execution. Frontend code is unchanged;
the existing v4-aligned component screenshots remain applicable. Exact-head CI
still performs the normal complete frontend and offline distribution guards.

Actual official Xray/Mihomo execution, syntax acceptance by those binaries, real
proxy/target/customer acceptance and non-Windows ownership acceptance are NOT_RUN.
No current authorization requires or permits an official core test run here.
End-to-end acceptance will need an approved single HTTPS target/status contract
and appropriate permission to run existing official cores and a controlled owned
proxy. New services are unnecessary for development and are not deployed.
