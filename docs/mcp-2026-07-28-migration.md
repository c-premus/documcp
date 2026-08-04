# MCP 2026-07-28 Migration Plan

**Status:** transport bump DONE on branch `feat/mcp-sdk-1.7.0` (2026-08-04) —
SDK at v1.7.0, unknown-tool e2e test updated, full MCP suite + unit suite + lint
green. REMAINING before prod: real-client verification (§4 Phase 2/6 — Claude.ai
connected service + `mcp-remote`) and the Renovate guard on `main` (§1). DEFERRED
as follow-ups: CIMD authorization (§4 Phase 4), optional gains (§4 Phase 5).
Retire this file to `memory-bank/archive/` once real-client verification passes.
**Written:** 2026-07-28
**Current runtime:** go-sdk v1.7.0, negotiating protocol `2026-07-28` (dual-era)

---

## 1. Act on this first: Renovate will ship the migration unattended

`go-sdk v1.7.0` released **2026-07-28** and implements protocol `2026-07-28`.
The bump from `v1.6.1` is a **minor** version, and `renovate.json` sets
`automerge: true` with `automerge: false` scoped to **major** updates only.

So on the current configuration:

1. `minimumReleaseAge: "3 days"` elapses (~2026-07-31).
2. Renovate raises the `v1.6.1 → v1.7.0` PR against `dev`.
3. CI goes green, Renovate self-merges.
4. `promote.yaml` fast-forwards `dev → main`, dispatches version-release.
5. `release.yaml` builds, dispatches `deploy.yaml`, production restarts.

A change of protocol era reaches production with no human in the loop, inside
about four days. That is not a hypothetical — it is the pipeline working as
designed on a dependency whose semver minor understates what it changes.

**Recommended mitigation** — add a package rule requiring manual approval for
the SDK that defines our wire protocol:

```json
{
  "description": "The MCP SDK defines our wire protocol; a minor bump can change protocol era (v1.6.1 -> v1.7.0 shipped protocol 2026-07-28). Never auto-merge it.",
  "matchPackageNames": ["github.com/modelcontextprotocol/go-sdk"],
  "automerge": false
}
```

Renovate reads `renovate.json` from the **default branch (`main`)**, so this
rule is inert until promoted to `main`. Land it there before 2026-07-31 or the
window closes.

---

## 2. What actually changed

The 2026-07-28 revision is the largest since launch. MCP became stateless *at
the protocol layer*:

| Area | 2025-11-25 | 2026-07-28 |
|---|---|---|
| Handshake | `initialize` / `notifications/initialized` | removed; per-request `_meta` |
| Sessions | `Mcp-Session-Id` header, DELETE to end | removed |
| Discovery | from the `initialize` result | `server/discover` RPC (servers **MUST** implement) |
| Version negotiation | once, at handshake | per request, via `_meta` + `MCP-Protocol-Version` |
| Routing | body only | `Mcp-Method` / `Mcp-Name` headers **REQUIRED** |
| Header/body disagreement | n/a | **MUST** reject, `400` + `-32020 HeaderMismatch` |
| GET stream | standalone SSE | removed; `405` |
| Server→client requests | JSON-RPC requests on SSE | Multi Round-Trip Requests (`InputRequiredResult`) |
| Change notifications | `notifications/*/list_changed` | `subscriptions/listen` long-lived stream |
| List results | — | cacheable via `ttlMs` / `cacheScope` |
| Roots, sampling, logging | supported | **deprecated** (≥12-month runway) |
| DCR (RFC 7591) | supported | **deprecated** in favor of Client ID Metadata Documents |

---

## 3. We are better positioned than the migration guide implies

The release notes are written for the median server, which is stateful and uses
sampling. Neither describes us. Verified against the v1.7.0 source, not the
blog post:

**The SDK is dual-era by construction.** `mcp/shared.go` in v1.7.0:

```go
latestProtocolVersion   = protocolVersion20260728
supportedProtocolVersions = []string{
    protocolVersion20260728, protocolVersion20251125,
    protocolVersion20250618, protocolVersion20250326,
}
```

A legacy client's `initialize` still negotiates down to `2025-11-25`; a modern
client gets `2026-07-28`. Both on the same handler, same endpoint.

**`server/discover` is already wired.** It sits in the SDK's own dispatch table
(`mcp/server.go`, `serverMethodInfos[methodDiscover]`) next to `initialize`. We
register nothing.

**Header validation is the SDK's job.** `mcp/streamable_headers.go` implements
`Mcp-Method` / `Mcp-Name` / `Mcp-Param-*` checking and emits
`CodeHeaderMismatch = -32020` on mismatch.

**The deprecations cost us nothing.** We already run
`StreamableHTTPOptions.Stateless: true`, every tool is pure request/response,
and `internal/handler/mcp/` contains no `sess.Notify`, sampling, elicitation,
roots, or resource subscribe. That precondition — recorded in
`systemPatterns.md` as the justification for stateless mode — is exactly what
MRTR and `subscriptions/listen` exist to replace. There is nothing to port.

**Net:** this is closer to a dependency bump plus verification than a rewrite.
The parts of the official migration guide that look alarming (remove session
handling, redesign server-initiated flows, migrate tasks) are all no-ops here.

---

## 4. Work items

### Phase 1 — SDK bump (small, mechanical)

- [ ] Bump `go-sdk` to `v1.7.0` **on a manual branch**, not via Renovate self-merge.
- [ ] Fix compile breaks. Known risk points in our code:
  - `Handler.Close()` iterates `h.server.Sessions()` and calls `sess.ID()` —
    verify both still exist once sessions are no longer a protocol concept.
  - The `Accept` header shim in `New()` (adds `text/event-stream` for Claude.ai
    clients sending only `application/json`) — confirm it is still needed and
    still harmless.
- [ ] Confirm no `MCPGODEBUG` escape hatch is required. All seven are removed in
  v1.9.0, so anything we need there is a deferred break, not a fix.
- [ ] Run the existing regression guards: `stateless_smoke_test.go`,
  `contract_regression_test.go`, `e2e_test.go`.

### Phase 2 — Dual-era verification (the actual risk)

- [ ] Extend `stateless_smoke_test.go` to assert **both** eras against one
      handler: a legacy `initialize` handshake negotiating `2025-11-25`, and a
      modern POST carrying `_meta` + `MCP-Protocol-Version: 2026-07-28`.
- [ ] Assert `server/discover` returns our tool set, and that `GET /documcp`
      still yields `405`.
- [ ] Assert a header/body mismatch yields `400` + `-32020`.
- [ ] Verify against a real client — Claude.ai connected service and
      `mcp-remote` — before promoting. Expect the `~/.mcp-auth` cache flush
      already noted in Known Issues.

### Phase 3 — Infrastructure

- [ ] Confirm Traefik forwards `MCP-Protocol-Version`, `Mcp-Method`, `Mcp-Name`,
      and `Mcp-Param-*` unmodified. A proxy that strips unknown headers turns
      every modern request into a `-32020`, and the failure would look like an
      SDK bug.
- [ ] Re-check the `/documcp` per-IP rate limit (60/min). Request *shape* does
      not change — we were already one POST per call — but confirm
      `server/discover` probes from dual-era clients do not eat the budget.

### Phase 4 — Authorization follow-through

Separate from the transport work, and the only genuinely new implementation:

- [ ] **Client ID Metadata Documents** (`draft-ietf-oauth-client-id-metadata-document-00`).
      Now a **SHOULD** for authorization servers; DCR is deprecated and retained
      only for backwards compatibility. Means accepting an HTTPS URL as
      `client_id`, fetching it, and validating the metadata + `redirect_uris`.
      Note the SSRF surface: this is a server-side fetch of a client-supplied
      URL, so it must go through the existing user-facing guard
      (private IPs blocked), **not** the admin `allowPrivate=true` path.
- [ ] Keep `/oauth/register` (DCR) working. Deprecated is not removed, and it
      has a ≥12-month runway.
- [ ] Consider `application_type` on registration (prevents localhost redirect
      rejection for native clients).

Already shipped ahead of this migration (2026-07-28): RFC 9207 `iss` emission +
`authorization_response_iss_parameter_supported`, `resource_metadata` and
`scope` in every `WWW-Authenticate`, and per-resource PRM `scopes_supported`.

### Phase 5 — Optional gains

- [ ] `ttlMs` / `cacheScope` on `tools/list` and `prompts/list`. Our tool list is
      static per process, so a generous TTL is free latency.
- [ ] Re-check `unified_search`'s discovery-only stance against the extensions
      framework; nothing forces a change.

### Phase 6 — Documentation

- [ ] `docs/contracts/mcp-contract.json`: `protocol_version`,
      `protocol_versions_accepted`, `sdk_version`.
- [ ] `techContext.md` MCP row; `systemPatterns.md` stateless-MCP section — the
      "no `sess.Notify` / sampling / elicitation" precondition graduates from a
      self-imposed constraint to what the protocol now assumes.
- [ ] Retire this file into `memory-bank/archive/` once complete.

---

## 5. Open decisions

1. **When.** The spec is one day old. Adopting a day-one revision into an
   unattended deploy pipeline is the main risk, and it is a scheduling choice,
   not a technical one. Waiting for a v1.7.x patch costs nothing while the SDK
   stays dual-era — legacy clients keep working either way.
2. **CIMD scope.** Full implementation, or advertise DCR only until a client
   actually requires CIMD? It is a SHOULD, and DCR has a 12-month runway.
3. **Whether to keep serving legacy after clients move.** Dual-era is free today
   because the SDK does it. Revisit only when `2025-11-25` is formally removed.

## 6. What is *not* urgent

Our current server is not broken. Per the spec's compatibility matrix, a
dual-era client against a legacy server **works** — the client falls back to
`initialize`. Only a *modern-only* client fails, and no shipping client is
modern-only while the SDKs default to dual-era. The deadline pressure in this
document comes from the Renovate window in §1, not from the protocol.
