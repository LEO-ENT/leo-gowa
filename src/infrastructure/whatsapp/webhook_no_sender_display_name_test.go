package whatsapp

// Contract (B1): no webhook payload may carry the key "sender_display_name",
// at any depth. The upstream enrichment exposed address-book names (privacy)
// and broke the leo-orchestrator user_text whitelist. Every builder below runs
// the production code path and the resulting JSON is walked recursively.

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
	"github.com/stretchr/testify/require"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

const forbiddenWebhookKey = "sender_display_name"

// findForbiddenWebhookKey returns the JSON paths where the forbidden key appears.
func findForbiddenWebhookKey(value any, path string) []string {
	var hits []string
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			childPath := path + "." + key
			if key == forbiddenWebhookKey {
				hits = append(hits, childPath)
			}
			hits = append(hits, findForbiddenWebhookKey(child, childPath)...)
		}
	case []any:
		for i, child := range typed {
			hits = append(hits, findForbiddenWebhookKey(child, path+"["+strconv.Itoa(i)+"]")...)
		}
	}
	return hits
}

func requireNoSenderDisplayName(t *testing.T, body any) {
	t.Helper()
	encoded, err := json.Marshal(body)
	require.NoError(t, err)
	var decoded any
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	hits := findForbiddenWebhookKey(decoded, "$")
	require.Empty(t, hits, "webhook payload leaks %q at %v: %s", forbiddenWebhookKey, hits, encoded)
}

func TestFindForbiddenWebhookKeyDetectsNestedKey(t *testing.T) {
	nested := map[string]any{"payload": map[string]any{"poll": []any{map[string]any{forbiddenWebhookKey: "x"}}}}
	require.Equal(t, []string{"$.payload.poll[0]." + forbiddenWebhookKey}, findForbiddenWebhookKey(nested, "$"))
}

func TestWebhookPayloadsNeverContainSenderDisplayName(t *testing.T) {
	ctx := context.Background()
	accountJID := types.NewJID("1234567890", types.DefaultUserServer)
	senderJID := types.NewJID("1234567891", types.DefaultUserServer)
	chatJID := types.NewJID("1234567892", types.DefaultUserServer)
	groupJID := types.NewJID("120363000000000000", types.GroupServer)
	client := &whatsmeow.Client{
		Store: &store.Device{
			ID:       &accountJID,
			PushName: "Test Account",
			Contacts: &senderDisplayNameContactGetter{
				contacts: map[types.JID]types.ContactInfo{
					senderJID: {Found: true, FullName: "Test Saved Contact"},
				},
			},
		},
	}
	ts := time.Date(2026, time.October, 12, 10, 0, 0, 0, time.UTC)

	messageEvent := func(sender types.JID, isFromMe bool, chat types.JID, msg *waE2E.Message) *events.Message {
		return &events.Message{
			Info: types.MessageInfo{
				MessageSource: types.MessageSource{
					Chat:     chat,
					Sender:   sender,
					IsFromMe: isFromMe,
					IsGroup:  chat.Server == types.GroupServer,
				},
				ID:        "B1-CONTRACT",
				PushName:  "Test Push Name",
				Timestamp: ts,
			},
			Message: msg,
		}
	}
	buildMessage := func(t *testing.T, evt *events.Message, poll ...*webhookPollPayload) any {
		t.Helper()
		webhookEvent, err := createWebhookEvent(ctx, client, evt, poll...)
		require.NoError(t, err)
		require.Equal(t, senderJID.String(), webhookEvent.Payload["from"], "case must exercise a sender-bearing payload")
		return webhookEvent
	}
	pollCreation := &waE2E.Message{PollCreationMessage: &waE2E.PollCreationMessage{
		Name: protoString("Test question"),
		Options: []*waE2E.PollCreationMessage_Option{
			{OptionName: protoString("A")},
			{OptionName: protoString("B")},
		},
		SelectableOptionsCount: func() *uint32 { v := uint32(1); return &v }(),
	}}

	tests := []struct {
		name  string
		build func(*testing.T) any
	}{
		{name: "message text inbound", build: func(t *testing.T) any {
			return buildMessage(t, messageEvent(senderJID, false, chatJID, &waE2E.Message{Conversation: protoString("hello")}))
		}},
		{name: "message text group", build: func(t *testing.T) any {
			return buildMessage(t, messageEvent(senderJID, false, groupJID, &waE2E.Message{Conversation: protoString("hello")}))
		}},
		{name: "message extended text", build: func(t *testing.T) any {
			return buildMessage(t, messageEvent(senderJID, false, chatJID, &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: protoString("hi")}}))
		}},
		{name: "message from me", build: func(t *testing.T) any {
			evt := messageEvent(accountJID, true, chatJID, &waE2E.Message{Conversation: protoString("hello")})
			webhookEvent, err := createWebhookEvent(ctx, client, evt)
			require.NoError(t, err)
			return webhookEvent
		}},
		{name: "media image", build: func(t *testing.T) any {
			return buildMessage(t, messageEvent(senderJID, false, chatJID, &waE2E.Message{ImageMessage: &waE2E.ImageMessage{
				Caption:  protoString("caption"),
				Mimetype: protoString("image/jpeg"),
			}}))
		}},
		{name: "media document", build: func(t *testing.T) any {
			return buildMessage(t, messageEvent(senderJID, false, chatJID, &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{
				FileName: protoString("file.pdf"),
				Mimetype: protoString("application/pdf"),
			}}))
		}},
		{name: "reaction", build: func(t *testing.T) any {
			return buildMessage(t, messageEvent(senderJID, false, chatJID, &waE2E.Message{ReactionMessage: &waE2E.ReactionMessage{
				Key:  &waCommon.MessageKey{RemoteJID: protoString(chatJID.String()), ID: protoString("target")},
				Text: protoString("👍"),
			}}))
		}},
		{name: "revoke", build: func(t *testing.T) any {
			return buildMessage(t, messageEvent(senderJID, false, chatJID, &waE2E.Message{ProtocolMessage: &waE2E.ProtocolMessage{
				Type: protoProtocolMessageType(waE2E.ProtocolMessage_REVOKE),
				Key:  &waCommon.MessageKey{ID: protoString("revoked")},
			}}))
		}},
		{name: "edit", build: func(t *testing.T) any {
			return buildMessage(t, messageEvent(senderJID, false, chatJID, &waE2E.Message{ProtocolMessage: &waE2E.ProtocolMessage{
				Type:          protoProtocolMessageType(waE2E.ProtocolMessage_MESSAGE_EDIT),
				Key:           &waCommon.MessageKey{ID: protoString("edited")},
				EditedMessage: &waE2E.Message{Conversation: protoString("updated")},
			}}))
		}},
		{name: "poll creation without prepared poll", build: func(t *testing.T) any {
			return buildMessage(t, messageEvent(senderJID, false, groupJID, pollCreation))
		}},
		{name: "poll creation with prepared poll", build: func(t *testing.T) any {
			evt := messageEvent(senderJID, false, groupJID, pollCreation)
			poll := preparePollWebhookPayload(ctx, client, newMemoryPollStore(), evt)
			require.NotNil(t, poll)
			return buildMessage(t, evt, poll)
		}},
		{name: "poll vote resolved", build: func(t *testing.T) any {
			creator := types.NewJID("1234567893", types.DefaultUserServer)
			voter := types.NewJID("1234567894", types.DefaultUserServer)
			pollID := types.MessageID("B1-POLL-1")
			crypto := newPollCryptoClient(t, "b1-no-sender-display-name-vote", voter)
			require.NoError(t, crypto.Store.MsgSecrets.PutMessageSecret(ctx, groupJID, creator, pollID, bytes.Repeat([]byte{0x42}, 32)))
			vote, err := crypto.BuildPollVote(ctx, &types.MessageInfo{
				MessageSource: types.MessageSource{Chat: groupJID, Sender: creator, IsGroup: true},
				ID:            pollID,
			}, []string{"A"})
			require.NoError(t, err)
			pollStore := newMemoryPollStore()
			require.NoError(t, pollStore.UpsertPollDefinition(&domainChatStorage.PollDefinition{
				DeviceID: voter.String(), ChatJID: groupJID.String(), PollMessageID: string(pollID),
				Question: "Test question", SelectableOptionCount: 1, Version: "v1",
				Options: []domainChatStorage.PollOption{{Name: "A", Hash: pollOptionHash("A")}, {Name: "B", Hash: pollOptionHash("B")}},
			}))
			evt := &events.Message{
				Info: types.MessageInfo{
					MessageSource: types.MessageSource{Chat: groupJID, Sender: voter, IsGroup: true},
					ID:            "B1-VOTE-1",
					PushName:      "Test Push Name",
					Timestamp:     ts,
				},
				Message: vote,
			}
			poll := preparePollWebhookPayload(ctx, crypto, pollStore, evt)
			require.NotNil(t, poll)
			require.Equal(t, "resolved", poll.ResolutionStatus)
			webhookEvent, err := createWebhookEvent(ctx, crypto, evt, poll)
			require.NoError(t, err)
			require.Equal(t, voter.String(), webhookEvent.Payload["from"])
			return webhookEvent
		}},
		{name: "receipt", build: func(*testing.T) any {
			return createReceiptPayload(ctx, &events.Receipt{
				MessageSource: types.MessageSource{Chat: chatJID, Sender: senderJID},
				MessageIDs:    []types.MessageID{"receipt"},
				Timestamp:     ts,
				Type:          types.ReceiptTypeRead,
			}, accountJID.String(), client)
		}},
		{name: "delete for me", build: func(t *testing.T) any {
			payload, err := createDeletePayload(ctx, &events.DeleteForMe{SenderJID: senderJID, MessageID: "deleted"}, nil, accountJID.String(), client)
			require.NoError(t, err)
			return payload
		}},
		{name: "chat presence", build: func(*testing.T) any {
			return createChatPresencePayload(ctx, &events.ChatPresence{
				MessageSource: types.MessageSource{Chat: chatJID, Sender: senderJID},
				State:         types.ChatPresenceComposing,
			}, accountJID.String(), client)
		}},
		{name: "call offer", build: func(*testing.T) any {
			return createCallOfferPayload(ctx, &events.CallOffer{
				BasicCallMeta: types.BasicCallMeta{CallCreator: senderJID, CallID: "call", Timestamp: ts},
			}, accountJID.String(), client, false)
		}},
		{name: "group participants", build: func(*testing.T) any {
			return createGroupInfoPayload(ctx, &events.GroupInfo{JID: groupJID, Timestamp: ts}, "join", []types.JID{senderJID}, accountJID.String(), client)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireNoSenderDisplayName(t, tt.build(t))
		})
	}
}
