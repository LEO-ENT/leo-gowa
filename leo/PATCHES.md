# LEO patch queue on upstream GOWA

Base tag: see `leo/BASE_TAG` (`v9.9.0` = `be8e3a2`). Source of the queue: `git log --reverse --no-merges up2/main..origin/main` on LEO-ENT/leo-gowa `main` (`f8b2ec3`).
Decisions follow SPEC `docs/design/2026-11-15-gowa-safety-and-upgrade-SPEC.md` B11 (LEO REBORN workspace).

`v9.9 sha` = commit on `leo/v9.9.0` (`-` = not carried, `(empty)` = no diff left on the tag).

Legend: **KEEP** = LEO patch carried on the tag · **DROP** = covered by upstream at the base tag · **UPSTREAMABLE** = carried (KEEP) until upstream merges an equivalent.

| sha | v9.9 sha | title | category | decision | reason |
|---|---|---|---|---|---|
| 3547696 | - | fix: resolve group message sender for MarkAsRead read receipts | markread | DROP | upstream `usecase/message.go` (group read sender resolution) |
| e360892 | - | address review: scope sender lookup to chat + structured logging (#772) | markread | DROP | follow-up of 3547696, upstream equivalent |
| 45280a8 | - | fix(media): scope downloads to active device (#1) | media | DROP | upstream `storedMessageForAction` / `GetMessageByIDChatAndDevice` |
| 576af45 | - | fix: keep the request port in generated URLs (#779) (#781) | rest | DROP | upstream #781 (`ee78e71`) in the tag |
| f8ba759 | 9a0ea11 | fix(webhook): auto-download document payload carries the sender filename | webhook-payload | KEEP | v9.9 auto-download document arm emits no `filename`; orch `channels/normalizer.py` reads `document.filename` (pinned by envelope golden) |
| 86345a6 | 76c165f | fix(webhook): fail closed per-device routing | webhook-routing | KEEP | LEO fail-closed per-device webhook routing |
| fe4d835 | c2e09c7 | test(webhook): pin Chatwoot on generic fail-closed | webhook-routing | KEEP | test of 86345a6 |
| e9565fe | (empty) | test(webhook): close generic fail-closed test scope | webhook-routing | KEEP (empty) | content already applied while resolving fe4d835 |
| 36ee197 | 263a70f | feat(webhook): add durable managed source spool | spool | KEEP | durable webhook spool (LEO) |
| 6b3bb78 | db46c20 | fix(webhook): harden spool startup and claim contention | spool | KEEP | spool hardening |
| b4f85d1 | 1c338c9 | ci: verify GoWA pull requests | ci | KEEP | fork CI |
| 6c941ca | e9249a5 | fix(ci): make GoWA verification portable | ci | KEEP | fork CI portability |
| 051b9f7 | c6344d2 | feat(send): support deterministic outbound text IDs | send | KEEP | deterministic outbound IDs used by orch |
| 8979e39 | a694d50 | fix(logging): redact Whatsmeow client events | logging-pii | KEEP | LEO typed PII boundary |
| 6e84a1e | a7d6a7d | fix(logging): harden WhatsApp PII boundaries | logging-pii | KEEP | LEO typed PII boundary |
| de90a03 | c27204f | fix(logging): redact Chatwoot receipt failures | logging-pii | KEEP | LEO typed PII boundary |
| 78ffc1b | b12fc2b | test(logging): enforce typed receipt boundary | logging-pii | KEEP | guard test |
| cb6f36e | f060087 | test(logging): harden typed receipt guard | logging-pii | KEEP | guard test |
| 75606c4 | dcc63b8 | test(logging): bound receipt alias closure | logging-pii | KEEP | guard test |
| d7bda59 | 535993d | feat(provider): add device-scoped message reconciliation | provider | KEEP | provider receipt reconciliation |
| cc7bb47 | b1301ed | docs(provider): publish reconciliation contract and CI gate | provider | KEEP | contract + CI gate |
| 153b953 | 8fead65 | fix(provider): harden receipt reconciliation boundary | provider | KEEP | provider hardening |
| 8ef4f3d | 5e8ed54 | fix(provider): enforce production middleware ordering | provider | KEEP | provider hardening |
| eccd6f9 | ec4a2cf | fix(provider): make lookup opacity path-owned | provider | KEEP | provider hardening |
| a709cfb | 30a9483 | fix(provider): bind lookup opacity to resolved route | provider | KEEP | provider hardening |
| d3752d0 | b6542c1 | fix(provider): deny lookup path by default | provider | KEEP | provider hardening |
| ed7b6d8 | 7f1a390 | test(provider): restore lookup call graph census | provider | KEEP | guard test |
| 2999fdd | (empty) | fix(merge): restore ErrInvalidJID dropped as deadcode by the markread line | merge-fix | KEEP (empty) | ErrInvalidJID restored while resolving 051b9f7 |
| 674a317 | - | feat(webhook): backport upstream poll vote selections (#816) onto the LEO fork | poll | DROP | upstream #816 poll implementation in v9.9; poll golden (6957ed8) green without it |
| eafe843 | - | fix(polls): recover definitions after SQLite contention | poll | DROP | depends on 674a317; upstream poll persistence used |
| 3796483 | - | fix: structure terminal poll persistence warnings | poll | DROP | depends on 674a317 |
| 7b62b23 | - | docs: align point five warning description | poll | DROP | docs of 674a317 |
| 88b12a0 | 0f6dc9c | fix(webhook): drop sender_display_name from every webhook payload (B1) | webhook-payload | KEEP | LEO privacy: no sender_display_name |
| 2a8e180 | 78844b2 | fix(groups): prove canonical PN from fresh participant roster (#6) | groups | UPSTREAMABLE | carried until upstream PR (F7) |
| f8b2ec3 | 1e669bc | feat(rest): GET /user/my/contacts/detailed (SPEC-031 S0-GOWA) (#7) | rest | UPSTREAMABLE | carried until upstream PR (F7) |

## Test commits added on the tag

| sha | title | why |
|---|---|---|
| 6957ed8 | test(leo): keep poll webhook contract on upstream implementation | poll golden (4 fixtures, byte-identical to the fork) green on upstream #816 -> poll group DROP |
| 86fa58d | test(leo): pin orchestrator calls without Origin through the v9.9 origin guard | v9.9 BrowserOriginGuard (#924) must let httpx calls (no Origin) through |
| 32dd744 | test(leo): golden webhook envelopes consumed by leo-orchestrator | F3-T5, see `leo/ENVELOPE_FIELDS.md` |
| (this) | test(leo): open usecase test DB with the build-selected sqlite driver | upstream v9.9.0 `usecase/message_test.go` hardcodes `sqlite3`: 27 purego failures on the pristine tag; UPSTREAMABLE |
| (ci) | ci(leo): run fork CI on the Go version required by v9.9.0 | `src/go.mod` = go 1.26.0 |
| c3bda68 | fix(webhook): device alerts fall back to the global webhook | decision of Frisco 2026-11-15, see below |

## LEO commits added on main after the v9.9.0 queue

Fork-only plumbing (category `leo-meta`, decision KEEP, never upstreamable): the weekly probe replays them like any other patch.

| sha | title | category | decision | reason |
|---|---|---|---|---|
| 1b93a4f | ci(leo): weekly upstream upgrade probe + generated orch endpoint list (GOWA F6-T1/T2) (#10) | leo-meta | KEEP | `.github/workflows/upstream-upgrade-probe.yml`; `ci.yaml` `workflow_call(ref)` + push on `upgrade/**`; `leo/probe/` (`upgrade-probe.sh`, `oasdiff_used.py`); `leo/orch-endpoints.txt` generated by leo-orchestrator `scripts/gowa_endpoints.py` |
| (this) | ci(leo): upgrade probe verdict as an issue instead of a PR | leo-meta | KEEP | GitHub Actions may not open PRs in the LEO-ENT org (org policy, 409): one issue per tag (`upstream-upgrade` label), `leo/probe/issue-verdict.sh` |

## Rebase notes (conflict resolutions)

Rule: LEO semantics on upstream structure; when upstream has an equivalent function it is extended, not duplicated. Commits not listed below were clean auto-merges.

- 86345a6: conflicts readme.md, webhook_forward.go; DU docs/per-device-webhook-plan.md (upstream deleted -> dropped). Extended upstream forwardPayloadWithWebhookConfig via new forwardPayloadWithResolvedWebhookConfig(resolveErr): lookup/validation error suppresses device+global legs (incl. merge-global). URL validation moved to validatedWebhookConfig() applied in getWebhookConfigForDevice, webhookConfigForPayload (companion AD path) and event_device instanceWebhookConfig (device.* events). Upstream test TestDeviceEventSQLBlockedLookupDoesNotHoldLogout inverted: expects no global fallback (LEO fail-closed).
- fe4d835: auto-merge dropped closing brace of the table-driven loop in webhook_forward_test.go; restored.
- e9565fe: EMPTY on v9.9 — its one-line brace fix was needed already while resolving fe4d835 (the original fe4d835 left the file uncompilable); skipped, content present.
- 36ee197: 8 event_*.go conflicts -> upstream structure kept, each `go func` forward replaced by LEO dispatchWebhookForward; upstream-new event_presence.go (#868) converted too. dispatchWebhookForward legacy path now uses context.WithoutCancel(ctx) (upstream v9.9 resolves per-slot webhook policy from ctx). webhook_forward.go: managed (fail-closed) delivery moved into forwardPayloadWithResolvedWebhookConfig, merge-global leg disabled in managed mode (no global/direct fallback). Ambiguous-companion warn log no longer prints device JID. MIGRATIONS: schema_info is positional; LEO spool = 44-46 (as in prod), upstream 44-54 shifted to 47+ with "(upstream N)" labels. upstream fixture usecase/testdata/chatstorage_v49.sql -> LEO variant chatstorage_v49_leo.sql (+spool tables, version shifted). cmd/mcp.go deleted upstream (MCP unified in rest) -> LEO change dropped; docs/per-device-webhook-plan.md deleted upstream -> dropped.
- 6c941ca: app_error.go conflict was pure gofmt alignment; kept upstream (already aligned incl. ErrScheduledSendNotFound). Added gofmt of upstream-unformatted chatwoot_content_test.go so the fork gofmt gate passes on v9.9.
- 051b9f7: extended upstream sendRequestExtras (ctx-pinned id, caller extra wins) instead of duplicating: wrapSendMessage = doSendMessage(ctx,..., sendRequestExtras(ctx, providerSendExtras(req)...)). MessageRequest keeps upstream AllowReshare + LEO provider fields. Upstream removed pkgError.ErrInvalidJID as deadcode -> restored here (content of 2999fdd, which becomes empty). Upstream test stub needed GetChatByDevice (v9.9 ephemeral lookup is device-scoped). Upstream mention-validation test aligned to LEO non-echo message. openapi/readme: upstream structure + LEO fields.
- 6e84a1e: upstream #926 (670be26) proxy logging = redactProxyURL + utils.FormatProxyParseFailure (scheme+host, device id). LEO typed boundary kept: init.go/device_manager.go call configureOutboundProxy (fixed event names only). Upstream redactProxyURL kept in proxy.go (its upstream tests). Upstream-new handleUndecryptableMessage log dropped SourceString() (sender JID) to satisfy the LEO PII census.
- 78ffc1b: upstream receipt filter shouldForwardReceipt (WHATSAPP_WEBHOOK_ALL_DEVICE_RECEIPTS) kept, LEO typed log logReceiptLinkedDeviceSkipped used in it. Upstream-new excluded-chat skips in syncReadReceiptsToChatwoot (one leaked the chat JID) routed through new typed helper logChatwootReceiptExcludedChat (added to receipt boundary rules).
- d7bda59: MIGRATIONS: LEO provider_message_receipts = position 47 (as in prod), upstream 44-54 now at 48-58 (identical to the v9.6 fork layout 48-53 for upstream 44-49, so prod schema_info 53 continues with upstream 50-54 at 54-58). chatstorage_wrapper: upstream poll wrappers + LEO provider wrappers side by side. Truncate/DeleteDevice delete both upstream (poll_definitions, scheduled_sends) and LEO (provider_message_receipts) tables. LEO v49 fixture now version 53 with provider table. Test stub got GetChatByDevice.
- cc7bb47: readme/openapi conflicts = upstream text structure kept; LEO provider bullet + /provider/messages/lookup path + ProviderMessageLookup{Request,Response} schemas re-inserted (openapi parsed with PyYAML after resolution).
- 153b953: rest.go = upstream MCP OAuth registration + BrowserOriginGuard (#924) kept; LEO shared accounts map + registerProviderLookupRoutes (own fail-closed basic auth + OpaqueDeviceMiddleware) registered before headerDeviceGroup. send.go: upstream normalizeStoredSender + LEO recordProviderMessagePresentAfterACK side by side.
- 8ef4f3d: registerProviderAndDeviceScopedRoutes placed where upstream registers the device group (LAST on apiGroup, after Chatwoot routes and the UI route, per upstream #837 invariant). Upstream AST test rest_route_order_test.go extended to treat that LEO helper call as the device-group position.
- 2999fdd: EMPTY on v9.9 — ErrInvalidJID already restored while resolving 051b9f7 (first commit that needs it on v9.9); skipped.
- 88b12a0: upstream addChatwootIdentityAliases kept in delete/receipt builders, addSenderDisplayName removed everywhere (no remaining non-test call site, incl. v9.7-v9.9 builders). Upstream TestBuildEventPayloadIncludesSenderDisplayName dropped; upstream album tests kept. webhook-payload.md: upstream rows kept minus sender_display_name rows. UPGRADE-NOTES.md deleted upstream -> dropped.
- f8ba759: clean pick after the golden comparison (see table).

## Schema migrations (positional `schema_info`)

| position | content |
|---|---|
| 1-43 | upstream 1-43 |
| 44-46 | LEO webhook spool (36ee197), as in prod |
| 47 | LEO provider_message_receipts (d7bda59), as in prod |
| 48-58 | upstream 44-54 ("Migration N (upstream M)") |

Prod is at 53 (= upstream 49 on the v9.6 fork); v9.9 adds upstream 50-54 at 54-58. To be validated on a prod DB copy in F4 (TEST.sh RUN1/RUN3).

## Decision 2026-11-15: device alerts are the only exception to fail-closed

Patch 86345a6 suppresses every generic webhook leg when the per-device webhook config cannot be
resolved (lookup error, 1s timeout, invalid URL). Decision of Frisco: `device.logged_out`,
`device.temporary_ban` and `device.reachout_timelock` instead go to the global `WHATSAPP_WEBHOOK`
(global secret and HMAC signature, global event whitelist), in legacy and managed mode alike, so
a ban/logout alert is never lost. Every other event stays fail-closed. Implemented in
`forwardDeviceAlertToGlobalWebhook` (`webhook_forward.go`) with a structured warn
(`event`, `reason=device_webhook_config_unavailable`, `targets` count; no JID, URL or secret).
Upstream test `TestDeviceEventSQLBlockedLookupDoesNotHoldLogout` is back to its upstream
assertion (global fallback). Tests: `device_alert_fallback_test.go`.
