# Store proxy-check offline candidate

Related to #22. Review the [draft native contract](store-proxy-check-contract.md)
before implementing the native adapter. This PR stages a real editor extension,
single-operation controller and synthetic fixture; **production does not supply
the optional port, so no real check is exposed or performed**. This is not Issue
#22 completion or native network acceptance. The only active Store editor change
is removing a hidden proxy-source column so the existing proxy input uses its
available width. Save, authorization, CAS and Store/generic boundaries are intact.

## Source and bounded visual evidence

The approved v4 HTML was re-hashed and actually rendered. SHA256:
`3a61a3c2386d264dabe7401835d4f6784fd22c73dc0f37db71315cf865a56efd`.
The network resources → owned networks → import-owned-proxy form was inspected
at the pixel level. Its fields, restrained secondary action, helper copy, border
palette and 44px controls guide this increment. The prototype's resource list,
purchases and simulated successful connections do not grant production features.
There is no exact three-phase probe result in the prototype: same-state pixel
equivalence for those rows is **NOT_RUN**. Overall shell/native visual acceptance
remains outside this change.

The optional component uses existing Form/Select/Button and scoped Store tokens.
No new dashboard/modal, country badge or merchant-login claim is introduced.
The full selected target is shown below its label and wraps on narrow screens.
New helper text uses the readable secondary token in both themes. Its secondary
buttons explicitly restore a solid border against the existing global button
reset. A before-fix fixture exposed the managed input's unused 220px source
column; the correction affects only managed mode.

All seven final PNGs were viewed after the last UI edit. These are real component
renders with synthetic data, not a running client or successful network probe:

| State | Evidence |
| --- | --- |
| Light, 1280×900, keyboard focus | [PNG](store-proxy-check-evidence/proxy-light.png) |
| Dark, 1280×900, target failed after TCP | [PNG](store-proxy-check-evidence/proxy-dark.png) |
| Light, 390×900, wrapping | [PNG](store-proxy-check-evidence/proxy-light-390.png) |
| Dark, 390×900, target remains untested | [PNG](store-proxy-check-evidence/proxy-dark-390.png) |
| Explicit cancellation, no success retained | [PNG](store-proxy-check-evidence/proxy-cancel.png) |
| Enterprise revision change, old evidence cleared | [PNG](store-proxy-check-evidence/proxy-scope.png) |
| Deadline, unknown result | [PNG](store-proxy-check-evidence/proxy-timeout.png) |

Adjacent JSON files record viewport geometry, fixed computed control styles and
safe counters. Every case: exactly one mock check despite duplicate activation,
zero saves/returns/unexpected native calls, zero external requests, zero runtime
exceptions and zero console errors. Both 390px renders have no horizontal page
overflow. Light check button: 44px high, solid `rgb(210, 220, 233)` border,
`rgb(86, 101, 122)` text, white background. Target selector: 44px high. Actual Tab
from the target selector focuses the check button with a visible solid outline.
The fixture runs under React StrictMode, including effect cleanup/remount.

## Behavior verification

- **218/218 frontend tests pass**, including 62 new contract/component cases.
- TypeScript plus clean production Vite build and whitespace check pass.
- New tests cover empty/illegal/oversize UTF-8/direct/unsupported input, explicit
  target selection, partial evidence, malformed/contradictory/mismatched DTOs,
  credential-bearing thrown errors and discarded diagnostic extras.
- Pending-operation tests cover duplicate activation, cancel and 12-second
  ceiling, holding the retry lock until adapter settlement, late success,
  unmount/stale callbacks, identity/generation/enterprise/revision/Store/binding/
  connector/input changes, and old timeout attribution before renderer effects.
- Independent review found a port-replacement lifetime gap. The controller now
  survives port changes and retains its pending lock until the old worker settles.
  A pure-controller case and two component cases cover ignored abort, delayed
  cleanup resolution/rejection, blocked reentry and dropped old results.
- Actual managed editor mock integration verifies no save, back, configuration
  mutation or native bridge call from checking. Removing the optional port hides
  the check action. Existing standalone and Store regression suites pass.
- Existing offline guards pass: launcher **44 cases**, distribution metadata
  **20 cases**, isolated Shell links **34 assertions in each PowerShell runtime**.
  Product/installer executions: **0**. Local installer fixture compilation was
  attempted but **NOT_RUN** because NSIS is absent; the existing CI compile guard
  must cover it. No dependency or compiler was installed for this change.

Reproduce the mock UI with the normal frontend Vite command and open
`/test/fixtures/store-proxy-check.html`. Select a target explicitly. Fixture
controls select synthetic outcomes; `.invalid` endpoints are never resolved.
Do not interpret these results as native authorization/transport proof.

## Integration and outstanding engineering

The branch is stacked on PR26 `b2ba5d566c02417b343194694fc6eaa0a8678b45` to reuse
the reviewed Store/editor theme. Only this increment belongs in its diff.
PR24 → PR26 is the existing visual dependency; PR27 is an independent root-route
change based on main and may be integrated separately. When integrating the
visual stack, preserve PR26's portal changes while retargeting its base after
PR24 lands. PR24/26/27 remain Draft and their exact-head CI runs were green at
this read-only check. None was modified, merged or deployed by this increment.

Before real activation, implement/review the dedicated native scoped binding,
existing identity/permission/Store/CAS admission, publisher target catalog,
absolute deadlines, cancellation acknowledgement, one-operation gate, sanitized
transport/error mapping and operation-owned cleanup. Audit existing connector
logging before reuse; do not weaken the generic Store guard or forward recent
logs. Native/real-proxy/Windows customer use, region and merchant login acceptance
are **NOT_RUN**. Independent contract/code review is still outstanding; keep Draft.
