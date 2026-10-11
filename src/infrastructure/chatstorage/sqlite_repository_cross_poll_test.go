package chatstorage

import (
	"context"
	"testing"
	"time"

	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
	"github.com/stretchr/testify/require"
)

func TestPollDefinitionRegisteredCreatorFallback(t *testing.T) {
	const receiver = "1234567890@s.whatsapp.net"
	const creator = "1234567891@s.whatsapp.net"
	const donor = "1234567892@s.whatsapp.net"
	const secondDonor = "1234567893@s.whatsapp.net"
	const group = "120363000000000@g.us"
	for _, scenario := range []string{"valid", "creator-unregistered", "receiver-unregistered", "donor-unregistered", "wrong-chat", "wrong-poll", "direct-chat", "slot-alias", "ambiguous-question", "ambiguous-options", "ambiguous-count", "ambiguous-version", "identical-latest"} {
		t.Run(scenario, func(t *testing.T) {
			repo := newTestSQLiteRepository(t)
			for i, jid := range []string{receiver, creator, donor, secondDonor} {
				if scenario == "creator-unregistered" && jid == creator || scenario == "receiver-unregistered" && jid == receiver || scenario == "donor-unregistered" && jid == donor {
					continue
				}
				require.NoError(t, repo.SaveDeviceRecord(&domainChatStorage.DeviceRecord{DeviceID: []string{"slot-receiver", "slot-creator", "slot-donor", "slot-second"}[i], JID: jid}))
			}
			stamp := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
			definition := &domainChatStorage.PollDefinition{DeviceID: donor, ChatJID: group, PollMessageID: "same-poll", Question: "Q?", Options: []domainChatStorage.PollOption{{Name: "A", Hash: "hash-a"}}, Version: "v3", SelectableOptionCount: 1, CreatedAt: stamp, UpdatedAt: stamp}
			require.NoError(t, repo.UpsertPollDefinition(definition))
			if scenario == "ambiguous-question" || scenario == "ambiguous-options" || scenario == "ambiguous-count" || scenario == "ambiguous-version" || scenario == "identical-latest" {
				second := *definition
				second.DeviceID = secondDonor
				second.UpdatedAt = stamp.Add(time.Hour)
				switch scenario {
				case "ambiguous-question":
					second.Question = "different"
				case "ambiguous-options":
					second.Options = []domainChatStorage.PollOption{{Name: "B", Hash: "hash-b"}}
				case "ambiguous-count":
					second.SelectableOptionCount = 2
				case "ambiguous-version":
					second.Version = "v2"
				}
				require.NoError(t, repo.UpsertPollDefinition(&second))
			}
			chat, poll, receiverID := group, "same-poll", receiver
			switch scenario {
			case "wrong-chat":
				chat = "120363999999999@g.us"
			case "wrong-poll":
				poll = "different-poll"
			case "direct-chat":
				chat = "1234567894@s.whatsapp.net"
			case "slot-alias":
				receiverID = "slot-receiver"
			}
			got, err := repo.GetPollDefinitionForRegisteredCreator(receiverID, creator, chat, poll)
			switch scenario {
			case "ambiguous-question", "ambiguous-options", "ambiguous-count", "ambiguous-version":
				require.Error(t, err)
				require.Nil(t, got)
				require.NotContains(t, err.Error(), donor)
			case "valid", "identical-latest":
				require.NoError(t, err)
				require.NotNil(t, got)
				require.Equal(t, group, got.ChatJID)
				require.Equal(t, definition.Options, got.Options)
				if scenario == "identical-latest" {
					require.Equal(t, secondDonor, got.DeviceID)
				} else {
					require.Equal(t, donor, got.DeviceID)
				}
			default:
				require.NoError(t, err)
				require.Nil(t, got)
			}
			// A successful fallback reads metadata without copying into the receiver scope.
			receiverDefinition, err := repo.GetPollDefinition(receiver, group, "same-poll")
			require.NoError(t, err)
			require.Nil(t, receiverDefinition)
			donorDefinition, err := repo.GetPollDefinition(donor, group, "same-poll")
			require.NoError(t, err)
			require.Equal(t, definition.Question, donorDefinition.Question)
			require.Equal(t, donor, donorDefinition.DeviceID)
		})
	}
}

func TestUpsertPollDefinitionContextCancellation(t *testing.T) {
	repo := newTestSQLiteRepository(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	definition := &domainChatStorage.PollDefinition{DeviceID: "1234567890@s.whatsapp.net", ChatJID: "120363000000000@g.us", PollMessageID: "cancelled", Question: "Q?"}
	require.ErrorIs(t, repo.UpsertPollDefinitionContext(ctx, definition), context.Canceled)
	got, err := repo.GetPollDefinition(definition.DeviceID, definition.ChatJID, definition.PollMessageID)
	require.NoError(t, err)
	require.Nil(t, got)
}
