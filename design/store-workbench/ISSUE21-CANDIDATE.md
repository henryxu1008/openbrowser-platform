# Issue 21 — first preparation UI candidate (cloud, not released)

Base: `03cc3451e2b392f9aeb748f81cd7f9d205acc958` (2026-10-30).
Scope: existing-authorized-store → prepare → save → explicitly open → close → reopen. This draft covers preparation and saving UX only, not release, deployment, account/Store/permission/proxy mutation or real browser operation.

## Difference from the existing implementation

| Customer step | Existing implementation reused | This candidate | Remaining evidence |
| --- | --- | --- | --- |
| Login / select authorized store | Native identity, StoreWorkspace context/catalog, server Store grants | No changes | No new Windows execution |
| Prepare the chosen kernel | Shared KernelVersionPicker, release catalog, checked immutable kernelcache | Remains visible, explicit selection; no automatic recommendation adoption | Real release/CDN preparation not run |
| Prepare environment | StoreEnvironmentOptions supplies original defaults or saved Profile | Show name/kernel/proxy first; history/memory/fingerprint only on explicit advanced expansion; retain original values | No new default fingerprint policy, region inference or automatic proxy selection |
| Save | Original managed adapter, bindingVersion/CAS, native authorization, Profile/binding/journal transaction | Block until options loaded; freeze edits during save; reject stale callback after unmount; keep input on failure and UNKNOWN blocked | No new ensure API, device metadata or state machine |
| Open / close / reopen | Existing explicit Store commands and owned-process cleanup | State-specific next-step guidance only, no command changes or automatic execution | Fresh-client native chain NOT_RUN; fixture is not process/persistence proof |
| Cancel / return | Existing dirty confirmation and managed return callback | Clear cancelled link target so a later return goes to the store | Deterministic component test, not GUI input evidence |

## Design and safety

Reuse existing `design/store-workbench/UI.md` U3/U4/U6 and existing Card/Form/Button layout. No new visual system, dependencies or backend contract.

Proxy remains an explicit visible local input with the existing direct-connection explanation. Existing proxy/fingerprint/history/memory values are not replaced on collapse, load failure or save failure. Preparing a kernel does not save or launch. A save does not launch. Failed/unknown native operations are not retried by this candidate.

The first slice is **not complete product acceptance**: this is a preparation UX candidate, not proof of installer delivery, actual remote distribution, real Store binding/permission/persistence or Chrome close/reopen. Formal release/configuration and native acceptance still require their own authorized window. Existing #21/#22/#23 and Commerce #1019/#1020/#1021 remain the tracking scope; this draft does not close Issue 21.

## Verification

Tests exercise actual editor/detail React components and a MemoryRouter; native adapters, kernel picker transport and view-only subcomponents are mocked. Initial seven cases: six RED, one existing UNKNOWN case GREEN. Added lifecycle guidance and unmounted save each separately reproduced RED before changes. All 97 frontend tests and the clean TypeScript/Vite build passed against this candidate. Test logs and build artifacts are not committed. The cancel-target regression was added after implementation, not claimed as a prior RED.

Independent read-only review found the unmounted-save gap; cleanup now invalidates the existing readiness ref and a regression covers it. Native Windows, Go backend changes/tests, real CDN, GUI screenshots, account operations, browser execution and cloud-to-desktop deployment: NOT_RUN. No backend source changed.

## Selected prototype integration slice (2026-10-31)

The Store list/detail adopts the selected light-blue list hierarchy without replacing production navigation or importing demo state. Catalog totals/status are server facts, not invented environment readiness. List errors have an explicit read retry; query controls follow history navigation and discard older reads. Existing prepare/save/open/close, enterprise revision, binding CAS and unknown-result recovery remain unchanged.

Network choice is bounded to the existing Store `direct_or_local_only` contract: reuse this Store's saved local configuration, enter a self-owned proxy address, or explicitly select direct. No global proxy pool, purchased inventory, automatic connectivity assertion or server extension is added. Clearing a custom address blocks save instead of silently switching to direct. No save or selection opens a browser.

See root `design-qa.md`: reference image/source inspected, but browser-rendered visual QA is **blocked / NOT_RUN**. This candidate is not a completed visual delivery. Marketplace, procurement approvals, network accounting and task budgets remain outside this PR.
