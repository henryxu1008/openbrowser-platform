# Historical draft: Store-owned proxy check

This records the PR28 frontend stage. The implemented native candidate and its
current limits are documented in [Store proxy native QA](store-proxy-native-qa.md).
The descriptions below are the original pre-native contract, not current
activation status.

Related to #22. That increment was an executable, mock-only frontend contract/UI
candidate. **No production bridge or native network adapter is connected.** It
does not enable a real connectivity button in the released Store editor. Native
authorization, cancellation, connector cleanup and a credential-safe transport
must be reviewed and implemented before activation. Existing Store/generic guards
and existing PR24/26/27 are unchanged.

## Current boundary and reuse

`StoreEnvironmentOptions` already checks identity generation, enterprise revision,
`CanUse` (existing `store-browser` use permission), Store visibility and binding.
`storeAPI()` captures the auth epoch. The managed editor provides current saved,
self-owned or explicit direct network choices, and preserves dirty input on save
failure. Those checks and behavior remain authoritative.

Generic `BrowserProxy*` access cannot prove Store ownership and is correctly
rejected by `guardStoreGeneral`. Do not exempt it. `TestRealConnectivityWithRuntimeConfig`
appends fallback targets, so it cannot implement this single-target contract as-is.
`BuildProxyHTTPClient`/bridge paths can log underlying errors and share runtime
bridges; do not call them from a new Store adapter until credentials, operation
ownership, context cancellation and cleanup have been audited. Never pass
`diagnosticsDTO`, `recentLogs`, raw errors, response bodies, URLs or process output
to the renderer.

## Native activation contract (not implemented in this increment)

1. A proposed dedicated `StoreProxyCheck` binding uses `scopedStore(generation)`;
   identity/business admission is checked before delegation. The renderer supplies
   Store ID, current enterprise revision, expected binding version (`0` only when
   genuinely unbound), operation ID, current proxy input and one target ID.
   Native derives issuer/subject/organization from the current trusted Store
   scope, never from renderer identity fields. Recheck `CanUse`, Store visibility,
   binding CAS and current identity/revision immediately before each network
   phase and before returning evidence. A changed binding is a conflict, never a
   request to adopt the replacement. No new remote permission or grant is added.
2. Either inspect the exact current unsaved self-owned input, or resolve the exact
   current Store binding/version natively. Never accept a global proxy ID/profile
   ID, generic pool, session token, caller headers, kernel path or runtime handle.
   Empty input and `direct://` are rejected for a **proxy** check. No implicit
   direct fallback, save, profile creation, launch, prepare, binding write, report
   or automatic retry occurs. An unsupported check protocol does not invalidate
   or rewrite the existing saved configuration.
3. The user explicitly selects **one** target from an immutable publisher-owned
   catalog. No default-selected target, free URL, target array or settings write.
   Native resolves the ID again; reject missing/unknown/stale IDs. First candidate
   uses only a credential-free HTTPS URL with an exact expected status set;
   no redirects, alternate hosts, fallback targets, target credentials, response
   body or region/IP lookup. Publisher target approval is outstanding; fixtures
   use `.invalid` and never resolve it.
4. A single operation has an absolute 12-second ceiling, including admission,
   DNS, connect, TLS and response headers; TCP phase at most 3 seconds and proxy
   target phase at most 6 seconds within the remaining budget. Bound input to
   8192 bytes, one TCP connection per phase, one HTTP request, zero redirects and
   no body reads. Cancellation must close operation-owned sockets/idle transport
   and release any bridge lease; never stop a borrowed/shared bridge or browser.
   Native permits at most one outstanding check per owning session/Store. Cancel
   is not proof the worker stopped: retries remain locked until native settles.
5. Resolve using the configured connector stack only. `xray` means the existing
   Xray + sing-box combination; `mihomo` is separate. Never switch stacks or
   bootstrap/download a missing connector. The first offline candidate validates
   ordinary HTTP/HTTPS/SOCKS5 URI syntax only. Other protocols return
   `UNSUPPORTED_PROTOCOL` for this check, without claiming the browser cannot use
   them. Bridge-backed protocol activation needs its own cleanup review.
6. Native errors are mapped to fixed codes. No proxy URI, host, username/password,
   bearer token, target response body, exception message or recent logs in UI,
   logs, analytics or reports. Credentials exist only in the in-memory request
   and operation-owned dialer; never serialize/persist a request for diagnostics.
   Until that property is established, no native adapter is wired.

## Small result DTO and meaning

The draft schema is `store-proxy-check-v1`. Echo only the opaque request ID and
selected target ID. Three independent evidence fields use `passed`, `failed`,
`not_checked`: `syntax`, `tcp`, `target`. The UI copies an explicit whitelist and
discards extra fields. Invalid/contradictory results become `UNAVAILABLE`.

- `syntax=passed`: supported URI format parsed; credentials not authenticated.
- `tcp=passed`: the proxy server accepted TCP at observation time; it says nothing
  about credentials, proxy handshake, exit region or target access.
- `target=passed`: the one explicit target produced the expected status through
  this proxy at that time. No general availability, location, account login,
  merchant acceptance, safety or risk-control conclusion follows.
- Fixed reasons: `NONE`, `INVALID_PROXY`, `UNSUPPORTED_PROTOCOL`,
  `TCP_UNREACHABLE`, `TARGET_UNREACHABLE`, `AUTH_REJECTED`, `TIMEOUT`, `CANCELED`,
  `UNAVAILABLE`, `PERMISSION_DENIED`, `CONTEXT_CHANGED`. Authentication failure is
  used only with positive protocol evidence, never inferred from timeout/DNS/TLS.

The controller owns a single operation. Changing identity/generation, enterprise
revision, Store, binding version, connector or input invalidates old evidence and
cancels the old request. Stale completion never reappears. Cancel/timeout ignores
late success and holds the retry lock until adapter settlement. Unmount aborts,
removes listeners and cannot publish a result. Renderer checks are UI fences,
**not native authorization**.
Replacing an adapter also invalidates and cancels the old operation, while the
same controller retains the lock until its worker actually settles. Native must
add a session/Store gate and operation-keyed cancellation across real remounts.

## Prototype fit and tests

The verified v4 HTML is the visual authority (SHA256
`3a61a3c2386d264dabe7401835d4f6784fd22c73dc0f37db71315cf865a56efd`).
Reuse the existing network form, secondary action, inline helper text, borders,
spacing and theme tokens. The prototype has no three-phase real-probe result;
show plain evidence/unknown rows rather than inventing a new dashboard, regional
badge, simulated purchase or successful network state. Fixture-only labels make
all synthetic results explicit. The component remains unmounted in production.

Offline tests cover empty/illegal/direct/unsupported input, target selection,
success and partial evidence, malformed DTOs, thrown credential-bearing errors,
cancel/timeout with late completion, duplicate actions, disposal, all scope/input
changes, no cross-stack switching and no save/start/report side effects. Real
proxy/network/native acceptance is **NOT_RUN**. Native implementation/review and
publisher target approval remain prerequisites for activation.
