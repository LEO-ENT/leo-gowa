package whatsapp

// LEO envelope contract (F3-T5, lane L-gowa). Golden webhook envelopes for every
// event leo-orchestrator consumes, produced through the production forwarding path
// (forwardMessageToWebhook / forwardGroupInfoToWebhook / forwardDeviceEvent) with
// the network submit stubbed. The field list is documented in leo/ENVELOPE_FIELDS.md.
//
// The comparison is on SHAPE, not on values: every key present in a golden file must
// still be present with the same JSON type (object, array, string, number, bool,
// null). Extra keys emitted by a newer upstream are allowed. On top of the golden
// shape, assertLeoRequiredPaths pins the exact keys the orchestrator reads.
//
// Regenerate fixtures with: UPDATE_LEO_ENVELOPE_FIXTURES=1 go test -run TestLeoEnvelopeContract

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/stretchr/testify/require"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

const leoEnvelopeFixtureDir = "testdata/leo_envelope_contract"

var (
	leoDeviceJID  = types.NewJID("393330000001", types.DefaultUserServer)
	leoPeerJID    = types.NewJID("393330000002", types.DefaultUserServer)
	leoMemberJID  = types.NewJID("393330000003", types.DefaultUserServer)
	leoGroupJID   = types.NewJID("120363000000000001", types.GroupServer)
	leoFixedClock = time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)
)

const leoSessionID = "leo-session"

type leoEnvelopeCase struct {
	name     string
	event    string
	required []string
	produce  func(t *testing.T) map[string]any
}

func TestLeoEnvelopeContract(t *testing.T) {
	cases := []leoEnvelopeCase{
		{
			name:  "message_dm_text",
			event: EventTypeMessage,
			required: []string{
				"event", "device_id", "session_id", "payload.id", "payload.body", "payload.from",
				"payload.chat_id", "payload.timestamp", "payload.is_from_me",
			},
			produce: func(t *testing.T) map[string]any {
				return leoForwardMessage(t, "dm-text", &events.Message{
					Info:    leoMessageInfo(leoPeerJID, leoPeerJID, false, "LEO-DM-TEXT-1"),
					Message: &waE2E.Message{Conversation: protoString("Ciao LEO")},
				})
			},
		},
		{
			name:  "message_group_reply",
			event: EventTypeMessage,
			required: []string{
				"event", "device_id", "session_id", "payload.id", "payload.body", "payload.from",
				"payload.chat_id", "payload.timestamp", "payload.is_from_me",
				"payload.replied_to_id", "payload.quoted_body",
			},
			produce: func(t *testing.T) map[string]any {
				return leoForwardMessage(t, "group-reply", &events.Message{
					Info: leoMessageInfo(leoGroupJID, leoMemberJID, false, "LEO-GROUP-REPLY-1"),
					Message: &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{
						Text: protoString("Confermo"),
						ContextInfo: &waE2E.ContextInfo{
							StanzaID:      protoString("LEO-GROUP-ORIGINAL-1"),
							Participant:   protoString(leoPeerJID.String()),
							QuotedMessage: &waE2E.Message{Conversation: protoString("Riunione alle 10?")},
						},
					}},
				})
			},
		},
		{
			name:  "message_image_caption",
			event: EventTypeMessage,
			required: []string{
				"event", "device_id", "session_id", "payload.id", "payload.from", "payload.chat_id",
				"payload.timestamp", "payload.image",
			},
			produce: func(t *testing.T) map[string]any {
				disableAutoDownloadMedia(t)
				caption := "Scontrino"
				return leoForwardMessage(t, "image", &events.Message{
					Info:    leoMessageInfo(leoPeerJID, leoPeerJID, false, "LEO-IMAGE-1"),
					Message: &waE2E.Message{ImageMessage: &waE2E.ImageMessage{Caption: &caption}},
				})
			},
		},
		{
			name:  "message_document",
			event: EventTypeMessage,
			required: []string{
				"event", "device_id", "session_id", "payload.id", "payload.from", "payload.chat_id",
				"payload.timestamp", "payload.document", "payload.document.filename",
			},
			produce: func(t *testing.T) map[string]any {
				disableAutoDownloadMedia(t)
				return leoForwardMessage(t, "document", &events.Message{
					Info: leoMessageInfo(leoPeerJID, leoPeerJID, false, "LEO-DOCUMENT-1"),
					Message: &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{
						FileName: protoString("preventivo.pdf"),
					}},
				})
			},
		},
		{
			name:  "message_revoked",
			event: EventTypeMessageRevoked,
			required: []string{
				"event", "device_id", "session_id", "payload.id", "payload.from", "payload.chat_id",
				"payload.timestamp", "payload.is_from_me", "payload.revoked_message_id", "payload.revoked_chat",
			},
			produce: func(t *testing.T) map[string]any {
				return leoForwardMessage(t, "revoked", &events.Message{
					Info: leoMessageInfo(leoGroupJID, leoMemberJID, false, "LEO-REVOKE-1"),
					Message: &waE2E.Message{ProtocolMessage: &waE2E.ProtocolMessage{
						Type: protoProtocolMessageType(waE2E.ProtocolMessage_REVOKE),
						Key: &waCommon.MessageKey{
							RemoteJID: protoString(leoGroupJID.String()),
							FromMe:    protoBool(false),
							ID:        protoString("LEO-GROUP-ORIGINAL-1"),
						},
					}},
				})
			},
		},
		{
			name:  "message_edited",
			event: EventTypeMessageEdited,
			required: []string{
				"event", "device_id", "session_id", "payload.id", "payload.from", "payload.chat_id",
				"payload.timestamp", "payload.is_from_me", "payload.original_message_id",
			},
			produce: func(t *testing.T) map[string]any {
				return leoForwardMessage(t, "edited", &events.Message{
					Info: leoMessageInfo(leoPeerJID, leoPeerJID, false, "LEO-EDIT-1"),
					Message: &waE2E.Message{ProtocolMessage: &waE2E.ProtocolMessage{
						Type:          protoProtocolMessageType(waE2E.ProtocolMessage_MESSAGE_EDIT),
						Key:           &waCommon.MessageKey{RemoteJID: protoString(leoPeerJID.String()), ID: protoString("LEO-DM-TEXT-1")},
						EditedMessage: &waE2E.Message{Conversation: protoString("Ciao LEO, scusa")},
					}},
				})
			},
		},
		{
			name:     "group_participants",
			event:    "group.participants",
			required: []string{"event", "device_id", "timestamp", "payload.chat_id", "payload.type", "payload.jids"},
			produce: func(t *testing.T) map[string]any {
				got := leoCaptureWebhooks(t)
				require.NoError(t, forwardGroupInfoToWebhook(context.Background(), &events.GroupInfo{
					JID:       leoGroupJID,
					Timestamp: leoFixedClock,
					Join:      []types.JID{leoMemberJID},
				}, leoDeviceJID.String(), nil))
				return leoRecvEnvelope(t, got, "group.participants")
			},
		},
		{
			name:  "device_temporary_ban",
			event: EventTypeDeviceTemporaryBan,
			required: []string{
				"event", "device_id", "session_id", "timestamp", "payload.code", "payload.reason",
				"payload.expires_in_seconds", "payload.expires_at",
			},
			produce: func(t *testing.T) map[string]any {
				return leoForwardDevice(t, EventTypeDeviceTemporaryBan,
					temporaryBanPayload(&events.TemporaryBan{Code: events.TempBanBlockedByUsers, Expire: time.Hour}, leoFixedClock))
			},
		},
		{
			name:     "device_logged_out",
			event:    EventTypeDeviceLoggedOut,
			required: []string{"event", "device_id", "session_id", "timestamp", "payload.code", "payload.reason", "payload.on_connect"},
			produce: func(t *testing.T) map[string]any {
				return leoForwardDevice(t, EventTypeDeviceLoggedOut,
					loggedOutPayload(&events.LoggedOut{OnConnect: true, Reason: events.ConnectFailureLoggedOut}))
			},
		},
		{
			name:     "device_reachout_timelock",
			event:    EventTypeDeviceReachoutTimelock,
			required: []string{"event", "device_id", "session_id", "timestamp", "payload.is_active", "payload.ends_at"},
			produce: func(t *testing.T) map[string]any {
				evt := &events.NotifyAccountReachoutTimelock{IsActive: true, EnforcementType: "REACHOUT_TIMELOCK"}
				evt.TimeEnforcementEnds.Time = leoFixedClock.Add(24 * time.Hour)
				return leoForwardDevice(t, EventTypeDeviceReachoutTimelock, reachoutTimelockPayload(evt))
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			envelope := tc.produce(t)
			require.Equal(t, tc.event, envelope["event"])
			leoAssertGolden(t, filepath.Join(leoEnvelopeFixtureDir, tc.name+".json"), envelope)
			assertLeoRequiredPaths(t, envelope, tc.required)
		})
	}
}

// TestLeoAutoDownloadMediaShapes pins the media value shapes emitted when
// WHATSAPP_AUTO_DOWNLOAD_MEDIA is on (the download itself needs the network, so
// the payload builders are exercised directly with a fixed extraction result).
func TestLeoAutoDownloadMediaShapes(t *testing.T) {
	extracted := utils.ExtractedMedia{MediaPath: "statics/media/1731664800-abc.pdf", MimeType: "application/pdf", Caption: "Preventivo"}
	shapes := map[string]any{
		"document_with_caption": buildAutoDownloadDocumentPayload(extracted, "preventivo.pdf"),
		"image_with_caption":    buildAutoDownloadPayload(extracted),
		"image_without_caption": buildAutoDownloadPayload(utils.ExtractedMedia{MediaPath: "statics/media/1731664800-abc.jpg"}),
	}
	leoAssertGolden(t, filepath.Join(leoEnvelopeFixtureDir, "media_auto_download_shapes.json"), shapes)

	document, ok := shapes["document_with_caption"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "preventivo.pdf", document["filename"], "orch normalizer reads document.filename")
}

func leoMessageInfo(chat, sender types.JID, fromMe bool, id string) types.MessageInfo {
	return types.MessageInfo{
		MessageSource: types.MessageSource{Chat: chat, Sender: sender, IsFromMe: fromMe, IsGroup: chat.Server == types.GroupServer},
		ID:            types.MessageID(id),
		Timestamp:     leoFixedClock,
	}
}

// leoCaptureWebhooks reuses the device-event capture stub, maps the device JID to a
// session id (as the device manager does in production) and disables Chatwoot.
func leoCaptureWebhooks(t *testing.T) <-chan deliveredWebhook {
	t.Helper()
	got := captureDeviceWebhooks(t)
	originalChatwoot := config.ChatwootEnabled
	t.Cleanup(func() { config.ChatwootEnabled = originalChatwoot })
	config.ChatwootEnabled = false
	sessionIDForJIDFn = func(jid string) string {
		if jid == leoDeviceJID.String() {
			return leoSessionID
		}
		return ""
	}
	return got
}

func leoForwardMessage(t *testing.T, dbName string, evt *events.Message) map[string]any {
	t.Helper()
	got := leoCaptureWebhooks(t)
	client := newPollCryptoClient(t, "leo-envelope-"+dbName, leoDeviceJID)
	require.NoError(t, forwardMessageToWebhook(context.Background(), client, evt))
	return leoRecvEnvelope(t, got, "")
}

func leoForwardDevice(t *testing.T, eventName string, payload map[string]any) map[string]any {
	t.Helper()
	got := leoCaptureWebhooks(t)
	inst := &DeviceInstance{id: leoSessionID, jid: leoDeviceJID.String(), createdAt: leoFixedClock}
	forwardDeviceEvent(context.Background(), eventName, inst, payload)
	return leoRecvEnvelope(t, got, eventName)
}

// leoRecvEnvelope returns the first delivery for the LEO test device (and, when
// event is set, for that event). The submit stub is package-global, so a late
// asynchronous delivery from another test may land on the same channel: it is
// skipped instead of being compared with the golden.
func leoRecvEnvelope(t *testing.T, got <-chan deliveredWebhook, event string) map[string]any {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case delivered := <-got:
			if delivered.body["device_id"] != leoDeviceJID.String() {
				continue
			}
			if event != "" && delivered.body["event"] != event {
				continue
			}
			return delivered.body
		case <-deadline:
			t.Fatal("timed out waiting for the LEO envelope")
			return nil
		}
	}
}

func leoAssertGolden(t *testing.T, fixture string, value any) {
	t.Helper()
	encoded, err := json.MarshalIndent(value, "", "  ")
	require.NoError(t, err)
	encoded = append(encoded, '\n')
	if os.Getenv("UPDATE_LEO_ENVELOPE_FIXTURES") == "1" {
		require.NoError(t, os.MkdirAll(filepath.Dir(fixture), 0o755))
		require.NoError(t, os.WriteFile(fixture, encoded, 0o644))
	}
	golden, err := os.ReadFile(fixture)
	require.NoError(t, err, "missing golden fixture; run with UPDATE_LEO_ENVELOPE_FIXTURES=1")

	var want, have any
	require.NoError(t, json.Unmarshal(golden, &want))
	require.NoError(t, json.Unmarshal(encoded, &have))
	var drift []string
	leoCompareShape("$", want, have, &drift)
	sort.Strings(drift)
	require.Empty(t, drift, "webhook envelope drifted from golden fixture %s", fixture)
}

// leoCompareShape records every golden key that is missing or changed JSON type.
func leoCompareShape(path string, want, have any, drift *[]string) {
	if leoJSONType(want) != leoJSONType(have) {
		*drift = append(*drift, fmt.Sprintf("%s: type %s -> %s", path, leoJSONType(want), leoJSONType(have)))
		return
	}
	switch w := want.(type) {
	case map[string]any:
		h := have.(map[string]any)
		for key, wantValue := range w {
			haveValue, ok := h[key]
			if !ok {
				*drift = append(*drift, fmt.Sprintf("%s.%s: missing", path, key))
				continue
			}
			leoCompareShape(path+"."+key, wantValue, haveValue, drift)
		}
	case []any:
		h := have.([]any)
		if len(w) > 0 && len(h) > 0 {
			leoCompareShape(path+"[0]", w[0], h[0], drift)
		}
	}
}

func leoJSONType(value any) string {
	switch value.(type) {
	case nil:
		return "null"
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "bool"
	default:
		return fmt.Sprintf("%T", value)
	}
}

// assertLeoRequiredPaths checks the keys leo-orchestrator reads (see
// leo/ENVELOPE_FIELDS.md): present and non-null.
func assertLeoRequiredPaths(t *testing.T, envelope map[string]any, paths []string) {
	t.Helper()
	encoded, err := json.Marshal(envelope)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	for _, path := range paths {
		var current any = decoded
		for _, part := range strings.Split(path, ".") {
			object, ok := current.(map[string]any)
			require.True(t, ok, "%s: parent of %q is not an object", path, part)
			current, ok = object[part]
			require.True(t, ok, "orchestrator field %s missing from envelope", path)
		}
		require.NotNil(t, current, "orchestrator field %s is null", path)
	}
}
