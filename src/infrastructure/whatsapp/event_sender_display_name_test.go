package whatsapp

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

type callOfferLIDStore struct {
	store.NoopStore
	phoneNumbers map[types.JID]types.JID
}

func (s *callOfferLIDStore) GetPNForLID(_ context.Context, lid types.JID) (types.JID, error) {
	return s.phoneNumbers[lid], nil
}

func TestCreateCallOfferPayloadNormalizesCaller(t *testing.T) {
	originalLog := log
	log = waLog.Noop
	defer func() { log = originalLog }()

	accountJID := types.NewJID("628111111111", types.DefaultUserServer)
	lidCaller := types.NewJID("251556368777322", types.HiddenUserServer)
	phoneCaller := types.NewJID("628123456789", types.DefaultUserServer)
	client := &whatsmeow.Client{
		Store: &store.Device{
			ID: &accountJID,
			Contacts: &senderDisplayNameContactGetter{
				contacts: map[types.JID]types.ContactInfo{
					phoneCaller: {
						Found:    true,
						FullName: "Saved Caller",
					},
				},
			},
			LIDs: &callOfferLIDStore{
				phoneNumbers: map[types.JID]types.JID{
					lidCaller: phoneCaller,
				},
			},
		},
	}

	tests := []struct {
		name        string
		caller      types.JID
		wantFrom    string
		wantFromLID string
	}{
		{
			name:        "LID caller",
			caller:      lidCaller,
			wantFrom:    "628123456789@s.whatsapp.net",
			wantFromLID: "251556368777322@lid",
		},
		{
			name:     "non-LID caller",
			caller:   phoneCaller,
			wantFrom: "628123456789@s.whatsapp.net",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := createCallOfferPayload(context.Background(), &events.CallOffer{
				BasicCallMeta: types.BasicCallMeta{
					CallCreator: tt.caller,
					CallID:      "call-id",
					Timestamp:   time.Date(2026, time.July, 29, 10, 0, 0, 0, time.UTC),
				},
			}, accountJID.String(), client, false)
			payload := body["payload"].(map[string]any)

			assert.Equal(t, tt.wantFrom, payload["from"])
			if tt.wantFromLID == "" {
				assert.NotContains(t, payload, "from_lid")
			} else {
				assert.Equal(t, tt.wantFromLID, payload["from_lid"])
			}
		})
	}
}
