package whatsapp

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

// REST sends save the definition and whatsmeow message secret without relying
// on a creation echo. A later incoming events.Message must reach delivery.
func TestRESTSavedPollIncomingVoteReachesWebhook(t *testing.T) {
	oldURLs, oldEvents := config.WhatsappWebhook, config.WhatsappWebhookEvents
	oldClosed, oldMerge, oldCW := config.WhatsappWebhookDeviceFailClosed, config.WhatsappWebhookDeviceMergeGlobal, config.ChatwootEnabled
	oldReply, oldRead, oldDownload := config.WhatsappAutoReplyMessage, config.WhatsappAutoMarkRead, config.WhatsappAutoDownloadMedia
	oldSubmit, oldStorage, oldLog := submitWebhookFn, webhookStorageForTest, log
	defer func() {
		config.WhatsappWebhook, config.WhatsappWebhookEvents = oldURLs, oldEvents
		config.WhatsappWebhookDeviceFailClosed, config.WhatsappWebhookDeviceMergeGlobal, config.ChatwootEnabled = oldClosed, oldMerge, oldCW
		config.WhatsappAutoReplyMessage, config.WhatsappAutoMarkRead, config.WhatsappAutoDownloadMedia = oldReply, oldRead, oldDownload
		submitWebhookFn, webhookStorageForTest, log = oldSubmit, oldStorage, oldLog
	}()
	config.WhatsappWebhook = []string{"https://example.test/vote"}
	config.WhatsappWebhookEvents = nil
	config.WhatsappWebhookDeviceFailClosed, config.WhatsappWebhookDeviceMergeGlobal, config.ChatwootEnabled = false, false, false
	config.WhatsappAutoReplyMessage, config.WhatsappAutoMarkRead, config.WhatsappAutoDownloadMedia = "", false, false
	log = waLog.Noop
	webhookStorageForTest = func(string) (*domainChatStorage.DeviceRecord, error) { return nil, nil }

	ctx := context.Background()
	creator := types.NewJID("1234567890", types.DefaultUserServer)
	voter := types.NewJID("1234567891", types.DefaultUserServer)
	chat := types.NewJID("120363000000", types.GroupServer)
	pollID := types.MessageID("REST-POLL-1")
	receiver := newPollCryptoClient(t, "rest-poll-receiver", creator)
	sender := newPollCryptoClient(t, "rest-poll-sender", voter)
	secret := bytes.Repeat([]byte{0x42}, 32)
	require.NoError(t, receiver.Store.MsgSecrets.PutMessageSecret(ctx, chat, creator, pollID, secret))
	require.NoError(t, sender.Store.MsgSecrets.PutMessageSecret(ctx, chat, creator, pollID, secret))
	vote, err := sender.BuildPollVote(ctx, &types.MessageInfo{
		MessageSource: types.MessageSource{Chat: chat, Sender: creator, IsGroup: true}, ID: pollID,
	}, []string{"Yes"})
	require.NoError(t, err)
	repo := &messageHandlerRepoSpy{pollDefinition: &domainChatStorage.PollDefinition{
		DeviceID: creator.String(), ChatJID: chat.String(), PollMessageID: pollID,
		Question: "Saved by REST?", Options: []domainChatStorage.PollOption{{Name: "Yes", Hash: pollOptionHash("Yes")}},
	}}
	done := make(chan map[string]any, 1)
	submitWebhookFn = func(_ context.Context, payload map[string]any, _ string, _ *domainChatStorage.DeviceWebhookConfig) error {
		done <- payload
		return nil
	}
	handlerCtx, cancel := context.WithCancel(ctx)
	cancel()
	handleMessage(handlerCtx, &events.Message{
		Info:    types.MessageInfo{MessageSource: types.MessageSource{Chat: chat, Sender: voter, IsGroup: true, IsFromMe: false}, ID: "REST-VOTE-1", Timestamp: time.Now()},
		Message: vote,
	}, repo, receiver)
	select {
	case envelope := <-done:
		require.Equal(t, EventTypeMessage, envelope["event"])
		payload := envelope["payload"].(map[string]any)
		poll := payload["poll"].(*webhookPollPayload)
		require.Equal(t, "vote", poll.Type)
		require.Equal(t, pollID, poll.PollID)
		require.Equal(t, pollResolutionResolved, poll.ResolutionStatus)
		require.NotNil(t, poll.SelectedOptions)
		require.Equal(t, []string{"Yes"}, *poll.SelectedOptions)
	case <-time.After(2 * time.Second):
		t.Fatal("incoming vote never reached webhook submission")
	}
}

func TestPollVoteFailureWarningsDoNotLeakIdentifiers(t *testing.T) {
	logger := logrus.StandardLogger()
	oldOutput, oldLevel, oldFormatter := logger.Out, logger.Level, logger.Formatter
	var output bytes.Buffer
	logger.SetOutput(&output)
	logger.SetLevel(logrus.WarnLevel)
	logger.SetFormatter(&logrus.TextFormatter{DisableTimestamp: true})
	defer func() { logger.SetOutput(oldOutput); logger.SetLevel(oldLevel); logger.SetFormatter(oldFormatter) }()
	evt := &events.Message{Info: types.MessageInfo{
		MessageSource: types.MessageSource{Chat: types.NewJID("1234567890", types.DefaultUserServer)}, ID: "PRIVATE-MESSAGE-ID",
	}, Message: nil}
	// A nil client degrades to a vote payload, with a warning instead of silence.
	evt.Message = &waE2E.Message{PollUpdateMessage: &waE2E.PollUpdateMessage{
		PollCreationMessageKey: &waCommon.MessageKey{ID: proto.String("PRIVATE-POLL-ID")},
	}}
	payload := preparePollWebhookPayload(context.Background(), nil, nil, evt)
	require.Equal(t, pollResolutionDecryptFailed, payload.ResolutionStatus)
	warnPollWebhookDiscard(map[string]any{"payload": map[string]any{"poll": payload}}, "event_filtered")
	require.Contains(t, output.String(), "level=warning")
	require.Contains(t, output.String(), "missing client")
	require.Contains(t, output.String(), "event_filtered")
	require.NotContains(t, output.String(), "1234567890")
	require.NotContains(t, output.String(), "PRIVATE-")
}

func TestManagedVoteNeverUsesGlobalMergeFallback(t *testing.T) {
	oldClosed, oldMerge := config.WhatsappWebhookDeviceFailClosed, config.WhatsappWebhookDeviceMergeGlobal
	oldURLs, oldStorage, oldSubmit := config.WhatsappWebhook, webhookStorageForTest, submitWebhookFn
	defer func() {
		config.WhatsappWebhookDeviceFailClosed, config.WhatsappWebhookDeviceMergeGlobal = oldClosed, oldMerge
		config.WhatsappWebhook, webhookStorageForTest, submitWebhookFn = oldURLs, oldStorage, oldSubmit
	}()
	config.WhatsappWebhookDeviceFailClosed, config.WhatsappWebhookDeviceMergeGlobal = true, true
	config.WhatsappWebhook = []string{"https://example.test/global"}
	webhookStorageForTest = func(string) (*domainChatStorage.DeviceRecord, error) {
		return nil, errors.New("private backend detail")
	}
	calls := 0
	submitWebhookFn = func(context.Context, map[string]any, string, *domainChatStorage.DeviceWebhookConfig) error {
		calls++
		return nil
	}
	err := forwardPayloadToConfiguredWebhooks(context.Background(), map[string]any{
		"device_id": "1234567890@s.whatsapp.net", "payload": map[string]any{"poll": &webhookPollPayload{Type: "vote"}},
	}, EventTypeMessage)
	require.Error(t, err)
	require.Zero(t, calls)
}
