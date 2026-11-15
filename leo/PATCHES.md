# LEO patch queue on upstream GOWA

Base tag: see `leo/BASE_TAG` (`v9.9.0` = `be8e3a2`). Source of the queue: `git log --reverse --no-merges up2/main..origin/main` on LEO-ENT/leo-gowa `main` (`f8b2ec3`).
Decisions follow SPEC `docs/design/2026-11-15-gowa-safety-and-upgrade-SPEC.md` B11 (LEO REBORN workspace).

Legend: **KEEP** = LEO patch carried on the tag · **DROP** = covered by upstream at the base tag · **UPSTREAMABLE** = carried (KEEP) until upstream merges an equivalent.

| sha | title | category | decision | reason |
|---|---|---|---|---|
| 3547696 | fix: resolve group message sender for MarkAsRead read receipts | markread | DROP | upstream `usecase/message.go` (group read sender resolution) |
| e360892 | address review: scope sender lookup to chat + structured logging (#772) | markread | DROP | follow-up of 3547696, upstream equivalent |
| 45280a8 | fix(media): scope downloads to active device (#1) | media | DROP | upstream `storedMessageForAction` / `GetMessageByIDChatAndDevice` |
| 576af45 | fix: keep the request port in generated URLs (#779) (#781) | rest | DROP | upstream #781 (`ee78e71`) in the tag |
| f8ba759 | fix(webhook): auto-download document payload carries the sender filename | webhook-payload | TBD (golden) | filename comparison v9.9 vs orch reader |
| 86345a6 | fix(webhook): fail closed per-device routing | webhook-routing | KEEP | LEO fail-closed per-device webhook routing |
| fe4d835 | test(webhook): pin Chatwoot on generic fail-closed | webhook-routing | KEEP | test of 86345a6 |
| e9565fe | test(webhook): close generic fail-closed test scope | webhook-routing | KEEP | test of 86345a6 |
| 36ee197 | feat(webhook): add durable managed source spool | spool | KEEP | durable webhook spool (LEO) |
| 6b3bb78 | fix(webhook): harden spool startup and claim contention | spool | KEEP | spool hardening |
| b4f85d1 | ci: verify GoWA pull requests | ci | KEEP | fork CI |
| 6c941ca | fix(ci): make GoWA verification portable | ci | KEEP | fork CI portability |
| 051b9f7 | feat(send): support deterministic outbound text IDs | send | KEEP | deterministic outbound IDs used by orch |
| 8979e39 | fix(logging): redact Whatsmeow client events | logging-pii | KEEP | LEO typed PII boundary |
| 6e84a1e | fix(logging): harden WhatsApp PII boundaries | logging-pii | KEEP | LEO typed PII boundary |
| de90a03 | fix(logging): redact Chatwoot receipt failures | logging-pii | KEEP | LEO typed PII boundary |
| 78ffc1b | test(logging): enforce typed receipt boundary | logging-pii | KEEP | guard test |
| cb6f36e | test(logging): harden typed receipt guard | logging-pii | KEEP | guard test |
| 75606c4 | test(logging): bound receipt alias closure | logging-pii | KEEP | guard test |
| d7bda59 | feat(provider): add device-scoped message reconciliation | provider | KEEP | provider receipt reconciliation |
| cc7bb47 | docs(provider): publish reconciliation contract and CI gate | provider | KEEP | contract + CI gate |
| 153b953 | fix(provider): harden receipt reconciliation boundary | provider | KEEP | provider hardening |
| 8ef4f3d | fix(provider): enforce production middleware ordering | provider | KEEP | provider hardening |
| eccd6f9 | fix(provider): make lookup opacity path-owned | provider | KEEP | provider hardening |
| a709cfb | fix(provider): bind lookup opacity to resolved route | provider | KEEP | provider hardening |
| d3752d0 | fix(provider): deny lookup path by default | provider | KEEP | provider hardening |
| ed7b6d8 | test(provider): restore lookup call graph census | provider | KEEP | guard test |
| 2999fdd | fix(merge): restore ErrInvalidJID dropped as deadcode by the markread line | merge-fix | KEEP | see rebase notes |
| 674a317 | feat(webhook): backport upstream poll vote selections (#816) onto the LEO fork | poll | TBD (golden) | upstream #816 `poll_event.go` |
| eafe843 | fix(polls): recover definitions after SQLite contention | poll | TBD (golden) | depends on 674a317 |
| 3796483 | fix: structure terminal poll persistence warnings | poll | TBD (golden) | depends on 674a317 |
| 7b62b23 | docs: align point five warning description | poll | TBD (golden) | depends on 674a317 |
| 88b12a0 | fix(webhook): drop sender_display_name from every webhook payload (B1) | webhook-payload | KEEP | LEO privacy: no sender_display_name |
| 2a8e180 | fix(groups): prove canonical PN from fresh participant roster (#6) | groups | UPSTREAMABLE | carried until upstream PR (F7) |
| f8b2ec3 | feat(rest): GET /user/my/contacts/detailed (SPEC-031 S0-GOWA) (#7) | rest | UPSTREAMABLE | carried until upstream PR (F7) |
