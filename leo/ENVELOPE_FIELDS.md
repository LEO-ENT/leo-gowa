# Webhook envelope fields read by leo-orchestrator

Source of truth for the consumer side: `LEO-ENT/leo-orchestrator` `origin/main` 244b268e
(`channels/whatsapp/gowa_adapter.py`, `channels/normalizer.py`, `channels/entrypoint.py`,
`channels/wa_event_journal/extract.py`). Golden envelopes:
`src/infrastructure/whatsapp/testdata/leo_envelope_contract/*.json`, checked by
`src/infrastructure/whatsapp/leo_envelope_contract_test.go` (shape comparison: a golden key
must stay present with the same JSON type; extra upstream keys are allowed).

Regenerate: `UPDATE_LEO_ENVELOPE_FIXTURES=1 go test -run 'TestLeoEnvelopeContract|TestLeoAutoDownloadMediaShapes' ./infrastructure/whatsapp/`

## Envelope (every event)

| Field | Type | Read by |
|---|---|---|
| `event` | string | adapter dispatch (`message`, `message.revoked`, `message.edited`, `message.ack`, `group.participants`, …) |
| `device_id` | string (JID) | `source_device_jid` |
| `session_id` | string | `source_device_id` (added by `addWebhookSessionID`; device.* events set it from the slot id) |
| `payload` | object | inner event |
| `timestamp` | string RFC3339 | group.participants and device.* only; message events carry it inside `payload` |

## `message`

| Field | Type | Notes |
|---|---|---|
| `payload.id` | string | message id (entrypoint trace, dedup) |
| `payload.body` | string | text; `_is_user_text` = non-empty body and nothing else of substance |
| `payload.from`, `payload.chat_id` | string | sender / chat JID |
| `payload.chat_lid`, `payload.from_lid` | string | optional, when a LID mapping exists |
| `payload.timestamp` | string RFC3339 | must be tz-aware |
| `payload.is_from_me` | bool | direction |
| `payload.participant`, `payload.from_name` | string | optional |
| `payload.replied_to_id` (legacy `replied_id`) | string | reply target |
| `payload.quoted_body` (legacy `quoted_message`) | string | quoted text |
| `payload.contact`, `payload.contacts_array` | object / array | `{displayName, vcard, phone_number}` |
| `payload.poll` | object | pinned separately by `poll_webhook_contract_test.go` |
| `payload.audio`/`image`/`video`/`document`/`sticker` | object or string | see "Media shapes" |
| `payload.document.filename` | string | sender-chosen name (Nextcloud import) — LEO patch f8ba759 for the auto-download arm |

## `message.revoked`

`payload.id`, `from`, `chat_id`, `timestamp`, `participant` (optional), `is_from_me`,
`revoked_message_id`, `revoked_chat`.

## `message.edited`

`payload.id`, `from`, `chat_id`, `timestamp`, `participant` (optional), `is_from_me`,
`original_message_id`. The new `body` is emitted but deliberately not read (orch proposal 81).

## `message.ack`

`payload.ids` (array), `payload.receipt_type` (string). Not in the golden set: covered by the
provider receipt tests (`provider_receipt_test.go`, `event_receipt_test.go`).

## `group.participants`

Consumed at the adapter boundary and dropped (never a turn). Envelope `event` is enough;
`payload.chat_id`, `payload.type` (`join`/`leave`/`promote`/`demote`), `payload.jids` are pinned
for the event journal. The adapter logs `payload.get("action")` presence on the envelope: GoWA
has never emitted `action` (the field is `payload.type`), so that log is always `False`.

## device.* (ban / logout / timelock alerts, SPEC outcome)

| Event | Payload fields |
|---|---|
| `device.temporary_ban` | `code` (number), `reason` (string), `expires_in_seconds` (number), `expires_at` (RFC3339, when expire > 0) |
| `device.logged_out` | `code` (number), `reason` (string), `on_connect` (bool) |
| `device.reachout_timelock` | `is_active` (bool), `ends_at` (RFC3339, optional), `enforcement_type` (string, optional) |

Envelope: `event`, `device_id` (when paired), `session_id` (slot id), `timestamp`.
No orchestrator consumer yet on 244b268e; the fields above are the SPEC contract for the alerts.

## Media shapes

- Auto-download OFF: `{"url": ..., "caption": ...}` (image/video), `{"url", "filename"}` (document), `{"url"}` (audio/sticker).
- Auto-download ON: image/video `"<path>"` string without caption, `{"path", "caption"}` with caption;
  document always `{"path", "caption"?, "filename"?}`; audio/sticker `"<path>"` string.
- Known gap (pre-existing, identical on v9.6 `origin/main`): the orchestrator normalizer reads
  `media_path`/`mime_type` from the object shape, GoWA emits `path` and no mime type. The string
  shape works; a captioned image/document object resolves to an empty media path on the orch side.
