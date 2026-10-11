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
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

type registeredPollStore struct {
	domainChatStorage.IChatStorageRepository
	local                           *memoryPollStore
	registered                      map[string]bool
	donor                           *domainChatStorage.PollDefinition
	registryErr                     error
	calls                           int
	receiver, creator, chat, pollID string
}

func (s *registeredPollStore) UpsertPollDefinition(d *domainChatStorage.PollDefinition) error {
	return s.local.UpsertPollDefinition(d)
}
func (s *registeredPollStore) GetPollDefinition(d, c, p string) (*domainChatStorage.PollDefinition, error) {
	return s.local.GetPollDefinition(d, c, p)
}
func (s *registeredPollStore) GetPollDefinitionByIDAndDevice(d, p string) (*domainChatStorage.PollDefinition, error) {
	return s.local.GetPollDefinitionByIDAndDevice(d, p)
}
func (s *registeredPollStore) AppendPollOption(d, c, p string, o domainChatStorage.PollOption) error {
	return s.local.AppendPollOption(d, c, p, o)
}
func (s *registeredPollStore) CreateMessage(context.Context, *events.Message) error { return nil }
func (s *registeredPollStore) GetPollDefinitionForRegisteredCreator(receiver, creator, chat, pollID string) (*domainChatStorage.PollDefinition, error) {
	s.calls++
	s.receiver, s.creator, s.chat, s.pollID = receiver, creator, chat, pollID
	if s.registryErr != nil {
		return nil, s.registryErr
	}
	if !s.registered[receiver] || !s.registered[creator] || s.donor == nil || !s.registered[s.donor.DeviceID] || s.donor.ChatJID != chat || s.donor.PollMessageID != pollID {
		return nil, nil
	}
	return s.donor, nil
}

func TestRegisteredCreatorPollFallback(t *testing.T) {
	for _, tc := range []struct {
		name                                                                                                 string
		receiverIsCreator, creatorVotes, ownDefinition, noSecret, unregistered, registryError, noParticipant bool
		wantStatus                                                                                           string
		wantCalls                                                                                            int
	}{
		{name: "different_receiver", wantStatus: pollResolutionResolved, wantCalls: 1},
		{name: "creator_receives_donor_copy", receiverIsCreator: true, wantStatus: pollResolutionResolved, wantCalls: 1},
		{name: "from_me_is_voter_not_receiver", creatorVotes: true, wantStatus: pollResolutionResolved, wantCalls: 1},
		{name: "own_definition_wins", ownDefinition: true, wantStatus: pollResolutionResolved},
		{name: "unregistered_creator", unregistered: true, wantStatus: pollResolutionDefinitionMissing, wantCalls: 1},
		{name: "missing_secret", noSecret: true, wantStatus: pollResolutionDecryptFailed},
		{name: "registry_error", registryError: true, wantStatus: pollResolutionDefinitionMissing, wantCalls: 1},
		{name: "group_participant_missing", noParticipant: true, wantStatus: pollResolutionDecryptFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			creator := types.NewJID("1234567890", types.DefaultUserServer)
			voter := types.NewJID("1234567891", types.DefaultUserServer)
			receiverJID := types.NewJID("1234567892", types.DefaultUserServer)
			donorJID := types.NewJID("1234567893", types.DefaultUserServer)
			if tc.receiverIsCreator {
				receiverJID = creator
			}
			if tc.creatorVotes {
				voter = creator
			}
			chat := types.NewJID("120363000000", types.GroupServer)
			pollID := types.MessageID("REGISTERED-REST-POLL")
			sender := newPollCryptoClient(t, "registered-sender-"+tc.name, voter)
			receiver := newPollCryptoClient(t, "registered-receiver-"+tc.name, receiverJID)
			secret := bytes.Repeat([]byte{0x42}, 32)
			require.NoError(t, sender.Store.MsgSecrets.PutMessageSecret(ctx, chat, creator, pollID, secret))
			if !tc.noSecret {
				require.NoError(t, receiver.Store.MsgSecrets.PutMessageSecret(ctx, chat, creator, pollID, secret))
			}
			vote, err := sender.BuildPollVote(ctx, &types.MessageInfo{MessageSource: types.MessageSource{Chat: chat, Sender: creator, IsGroup: true, IsFromMe: tc.creatorVotes}, ID: pollID}, []string{"Yes"})
			require.NoError(t, err)
			if tc.noParticipant {
				vote.PollUpdateMessage.PollCreationMessageKey.Participant = proto.String("")
			}
			store := &registeredPollStore{local: newMemoryPollStore(), registered: map[string]bool{receiverJID.String(): true, creator.String(): true, donorJID.String(): true}, donor: &domainChatStorage.PollDefinition{DeviceID: donorJID.String(), ChatJID: chat.String(), PollMessageID: pollID, Question: "Only on registered donor", Options: []domainChatStorage.PollOption{{Name: "Yes", Hash: pollOptionHash("Yes")}}}}
			if tc.unregistered {
				delete(store.registered, creator.String())
			}
			if tc.registryError {
				store.registryErr = errors.New("PRIVATE-BACKEND-1234567890@s.whatsapp.net")
			}
			if tc.ownDefinition {
				def := *store.donor
				def.DeviceID = receiverJID.String()
				def.Question = "Own definition"
				require.NoError(t, store.UpsertPollDefinition(&def))
			}
			var warnings bytes.Buffer
			logger := logrus.StandardLogger()
			oldOutput, oldLevel := logger.Out, logger.Level
			logger.SetOutput(&warnings)
			logger.SetLevel(logrus.WarnLevel)
			defer func() { logger.SetOutput(oldOutput); logger.SetLevel(oldLevel) }()
			evt := &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: chat, Sender: voter, IsGroup: true}, ID: "REGISTERED-VOTE", Timestamp: time.Now()}, Message: vote}
			poll := preparePollWebhookPayload(ctx, receiver, store, evt)
			require.NotNil(t, poll)
			require.Equal(t, tc.wantStatus, poll.ResolutionStatus)
			require.Equal(t, tc.wantCalls, store.calls)
			if store.calls > 0 {
				require.Equal(t, receiverJID.String(), store.receiver)
				require.Equal(t, creator.String(), store.creator)
				require.Equal(t, chat.String(), store.chat)
				require.Equal(t, string(pollID), store.pollID)
			}
			if tc.wantStatus == pollResolutionResolved {
				require.Equal(t, []string{"Yes"}, *poll.SelectedOptions)
				if tc.ownDefinition {
					require.Equal(t, "Own definition", poll.Question)
				} else {
					require.Equal(t, store.donor.Question, poll.Question)
				}
				assertRegisteredPollForward(t, ctx, receiver, store, evt, receiverJID.String())
			} else {
				require.Contains(t, warnings.String(), "level=warning")
				require.NotContains(t, warnings.String(), "123456789")
				require.NotContains(t, warnings.String(), "PRIVATE-BACKEND")
			}
		})
	}
}

func assertRegisteredPollForward(t *testing.T, ctx context.Context, client *whatsmeow.Client, store *registeredPollStore, evt *events.Message, receiverID string) {
	// Deliberately keep real message handling and delivery admission in the test.
	// Only the final network operation is replaced.
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
	config.WhatsappWebhook = []string{"https://example.test/registered-vote"}
	config.WhatsappWebhookEvents = nil
	config.WhatsappWebhookDeviceFailClosed, config.WhatsappWebhookDeviceMergeGlobal, config.ChatwootEnabled = false, false, false
	config.WhatsappAutoReplyMessage, config.WhatsappAutoMarkRead, config.WhatsappAutoDownloadMedia = "", false, false
	log = waLog.Noop
	webhookStorageForTest = func(string) (*domainChatStorage.DeviceRecord, error) { return nil, nil }
	done := make(chan map[string]any, 1)
	submitWebhookFn = func(_ context.Context, payload map[string]any, _ string, _ *domainChatStorage.DeviceWebhookConfig) error {
		done <- payload
		return nil
	}
	handleMessage(ctx, evt, store, client)
	select {
	case envelope := <-done:
		require.Equal(t, receiverID, envelope["device_id"])
		poll := envelope["payload"].(map[string]any)["poll"].(*webhookPollPayload)
		require.Equal(t, pollResolutionResolved, poll.ResolutionStatus)
		require.Equal(t, []string{"Yes"}, *poll.SelectedOptions)
	case <-time.After(2 * time.Second):
		t.Fatal("resolved registered-device vote never reached webhook")
	}
}
