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
release the Store/session gate before the process and its I/O finish. Tests use
only a local process fixture. Production activation remains closed until its
owned core/config/readiness integration and target source are verified.

## Evidence and release status

The source loader's tests exercise malformed/ambiguous manifests, copied metadata,
version changes, safe empty defaults and attempted installed/user-settings
injection. No runtime health endpoint has been probed. Real proxy, region,
merchant-login and customer acceptance remain NOT_RUN. Keep Draft for independent
review; no merge or deployment.
