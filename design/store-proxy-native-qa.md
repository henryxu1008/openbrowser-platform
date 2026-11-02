# Scoped native Store proxy-check candidate

Related to #22, stacked on PR28 including its reviewed port-lifetime fix
`45e6df1c1e41adcf893cc1bcbd23c9120806dee4`. The production Store adapter now uses
dedicated native bindings. **No approved publisher target exists in this source:
the production catalog stays empty and checks fail closed.** The UI explains
the missing target configuration and identifies the package maintainer as the
next step. No external endpoint was selected, configured or probed for this work.
This is a native implementation candidate, not Issue22 completion or a ready
customer connectivity feature.

The following increment adds a compiled publisher source (still empty) and an
owned bridge lifecycle/real-child fixture. Its real core Starter remains unbound;
see [source, bridge contract and health-interface investigation](store-proxy-publisher-and-bridge.md).

## Authority and operation lifetime

- `StoreProxyCheck(storeID, expectedVersion, raw, revision, generation)` uses
  existing business admission and `scopedStore`, derives issuer/subject/enterprise
  natively, and checks the existing `CanUse` permission and authorized Store.
  The existing Store client verifies schema, Store ID, subject and organization
  in its response. No identity field or arbitrary profile/pool ID is accepted.
- Strict JSON permits only `requestId`, `proxyConfig`, `targetId`, `catalogVersion`
  and a caller expiry capped at twelve seconds. Expired dispatches cannot revive
  canceled intent after tombstone eviction or a suspend/resume cycle.
  Canonical nonzero UUID, 16KiB envelope and 8192-byte proxy limits apply. Unknown
  fields and malformed input return fixed errors, without echoing input.
- Cancellation ownership is reserved before remote admission. Native maintains
  a session gate and a Store gate, including across renderer remounts and a
  replacement login. Busy/binding results follow Store permission admission.
  Binding ID/profile ID/version are pinned and rechecked; the existing Store
  read lock prevents simultaneous Store save/open changes. No save, launch,
  preparation, report, credential persistence or journal intent is performed.
- `StoreProxyCheckCancel(storeID, requestId, originalRevision, generation)` can
  only stop its original session/Store/revision/operation. It does not need a new
  enterprise selection or use grant for owned cleanup. Identity changes cancel
  old work through a local 25ms watcher. Full permission/Store/CAS checks run at
  execution stages and before returning evidence.
- Cancel before dispatch installs an owner-bound tombstone. A canceled or expired
  worker retains both gates until all cleanup finishes; `settled=true` is emitted
  only afterward. A cancel wait can return `settled=false` after its deadline,
  which does not release the worker's gate. Private history contains no request
  credentials: at most 512 reservations/history entries, five-minute deduplication
  retention. A replay never triggers another check within that window.
- PR28's component controller now survives `port` replacement. Its original
  promise must settle before retry; old adapter success/rejection cannot refill
  the new view. Native gates also cover actual component unmount/remount.

## Catalog, connector boundaries and evidence

The existing user-editable `ProxyCheck` configuration and generic multi-target
connectivity helpers are deliberately not consumed. There is no renderer or
runtime setter for the private catalog. `NewCatalog` validates/copies a future
publisher-owned catalog: at most eight targets, unique IDs, bounded labels/URLs,
credential-free HTTPS DNS names on port443, no query/fragment/IP/localhost/local
targets, and a bounded positive expected-status set. An exact catalog fingerprint
and target ID must resolve again natively. Validation does not itself establish
publisher trust; approval and integration of a real source remain outstanding.

The supported network subset follows the existing connector matrix:

- HTTP/HTTPS/SOCKS5 without credentials use the existing native protocol path.
  A private transport uses standard `net/http` and the existing `x/net/proxy`
  SOCKS5 primitive; it has no shared connector manager or pool.
- Credentialed standard proxies require Xray/Mihomo in the browser's matrix.
  This candidate can record format and TCP evidence, then returns
  `CONNECTOR_REQUIRED` with credentials/target untested. It never authenticates
  through an alternative native path, borrows a shared bridge, switches stacks,
  downloads a core or launches a bridge. An owned bridge adapter is future work.
- Other bridge protocols remain unsupported for this check; format is unknown,
  not falsely declared invalid. Existing browser save/start support is unchanged.

The absolute work deadline is 12 seconds including admission; TCP is at most
3 seconds and target access at most 6 seconds within remaining time. HTTP has a
32KiB response-header bound and 256KiB total socket-read bound (including proxy
CONNECT/TLS handshakes), one target request, no redirects, no body reads,
no cookies or caller headers, no system proxy and no direct-target fallback.
Dialing is pinned to the explicit proxy endpoint. Connections, bodies and idle
transports are owned and closed before settlement. A late successful response
after phase cancellation/expiry is discarded. Delayed cleanup holds the gate.

The owned transport additionally seals late dial/read/write capabilities, cancels
and joins each active dial and socket I/O callback, and closes operation-owned
connections before settlement. `CloseIdleConnections` alone is insufficient:
Go1.26.5 can return from a canceled request while its dial is still unwinding.
A real `ownedTransport` backend regression reproduced premature settlement and a
third dial on `0be3cf5` (`check settled before actual dial cleanup`, followed by
`TCP_UNREACHABLE dials=3`). Delayed dial cleanup now retains both native gates,
returns Busy without a third dial, and acknowledges settlement only afterward.
Additional tests cover reads that delay cleanup, late connections returned
after cancellation, and denied I/O/dials after the seal.

Only the small v1 DTO crosses the bridge: schema/request ID/target ID, format,
TCP and target evidence plus fixed reason. No recent logs, raw errors, headers,
body, IP/region or merchant-login inference is returned. HTTP CONNECT 407 is
positive proxy-auth evidence; timeout/opaque SOCKS errors are not interpreted as
proven bad credentials. This implementation cannot prove browser-stack readiness,
region correctness, merchant acceptance or account safety.

## Credential and IPC handling

The new use case never calls the generic diagnostics or logging HTTP-client
builder. URI/parser/network errors are mapped to fixed codes; panics are caught
after cleanup and become a fixed unavailable error. No request is persisted.

Pinned Wails v2.12.0 dispatcher source logs raw IPC packets on parsing errors and
complete callback data at trace level. The existing lifecycle logger now omits
only these data-bearing IPC payload classes (including obfuscated calls), plus
Store proxy-check messages. Ordinary lifecycle events remain. Fixture log-file
tests confirm synthetic credentials are absent. This protects the IPC boundary
in addition to the use case's fixed errors, without changing business admission.

## Verified offline scope

The actual Store pages, editor, auth epoch capture, generated typed boundary and
SDK compatibility wrappers are covered by mock-bridge tests. The native suite
uses the real backend use case/DAO and injected Store/transport fixtures. No
customer account, service, browser process or external proxy was operated.

- **231/231 frontend tests** and TypeScript/clean Vite build pass. Tests cover
  exact Store/CAS/revision/generation arguments; malformed/missing catalog; direct
  mode; cancellation, input change and unmount; no save/open/close/prepare side
  effects; pinned catalog/scope/cancel keys; and bridge-required partial evidence.
- Full offline Go race suite passes: **1671 passed test events, including subtests**
  (**531 top-level tests**).
  Native cases cover Store/permission/identity/
  enterprise/binding changes, cancel during admission, owner/revision isolation,
  cross-remount and cross-login cleanup gates, session concurrency, phase deadlines
  with late success, no redirect/body read/direct fallback, catalog trust gates,
  protocol-matrix compatibility, strict errors and IPC log/panic sanitization.
- Existing launcher/distribution/shortcut guards pass locally; installer compile
  is delegated to the existing CI because local NSIS is absent. A Windows
  production executable is compiled for validation and never executed.
- The verified v4 source hash remains
  `3a61a3c2386d264dabe7401835d4f6784fd22c73dc0f37db71315cf865a56efd`.
  Its existing network form/tokens guide this extension. It has no exact real
  three-phase check or missing-publisher-target state; same-state pixel identity
  is NOT_RUN. No new navigation, purchase or regional success design is invented.

All six screenshots below were viewed. They are real component renders with
synthetic services, not actual native network success. Adjacent JSON records safe
counters/geometry: zero save/open/unexpected native calls, external requests,
runtime exceptions and console errors; 390px states have no horizontal overflow.
The default unconfigured cases issue zero check calls. Synthetic configured
examples issue one mock check despite repeated activation. Cancellation and
enterprise change cancel the original operation; enterprise change returns to
the existing list without retaining old evidence. Keyboard Tab reaches the check
button with a visible outline.

| State | Evidence |
| --- | --- |
| Default missing publisher target, light 1280×900 | [PNG](store-proxy-native-evidence/native-missing-light.png) |
| Default missing publisher target, dark 390×900 | [PNG](store-proxy-native-evidence/native-missing-dark-390.png) |
| Synthetic bridge response, light 1280×900 | [PNG](store-proxy-native-evidence/native-bridge-light.png) |
| Synthetic target remains untested, dark 390×900 | [PNG](store-proxy-native-evidence/native-bridge-dark-390.png) |
| Original operation canceled | [PNG](store-proxy-native-evidence/native-bridge-cancel.png) |
| Enterprise change returns to list | [PNG](store-proxy-native-evidence/native-bridge-scope.png) |

Run the normal frontend Vite fixture at `/test/fixtures/store-proxy-native.html`.
Its default models the empty production catalog. `?catalog=fixture` injects an
explicitly synthetic `.invalid` catalog into mock endpoints only; this is not a
production configuration option and performs no network traffic.

## Remaining gates

Keep Draft for independent native/contract review. A trusted publisher target
source, an operation-owned connector adapter for authenticated/bridge protocols,
and real Windows/customer/target acceptance are outstanding. No actual proxy,
region or merchant-login acceptance was run. Existing entitlement/role gates and
generic Store boundaries are unchanged. No merge, deployment or release.
