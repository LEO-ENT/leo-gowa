package whatsapp

// Contract test for the poll-vote webhook consumed by leo-orchestrator
// (LEO-ENT/leo-orchestrator PR #808: channels/whatsapp/gowa_adapter.py reads
// payload["poll"] and payload["timestamp"]; legal/polls.py validates
// type, poll_id, resolution_status, question, options[].{name,hash},
// selected_options and selected_option_hashes).
//
// Every case builds a REAL encrypted vote with whatsmeow, runs the production
// path (preparePollWebhookPayload -> createWebhookEvent) and compares the
// resulting JSON with the golden fixture in testdata/poll_webhook_contract.
// Regenerate fixtures with: UPDATE_POLL_CONTRACT_FIXTURES=1 go test -run TestPollVoteWebhookContract

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
	"github.com/stretchr/testify/require"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

const pollContractFixtureDir = "testdata/poll_webhook_contract"

type pollContractCase struct {
	name           string
	selected       []string
	storeDef       bool
	badSecret      bool
	wantStatus     string
	wantSelections bool // selected_options / selected_option_hashes keys present
	wantDefinition bool // question / options keys present
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func TestPollVoteWebhookContract(t *testing.T) {
	cases := []pollContractCase{
		{name: "resolved", selected: []string{"Accetto"}, storeDef: true, wantStatus: "resolved", wantSelections: true, wantDefinition: true},
		{name: "vote_cleared", selected: []string{}, storeDef: true, wantStatus: "resolved", wantSelections: true, wantDefinition: true},
		{name: "definition_missing", selected: []string{"Accetto"}, storeDef: false, wantStatus: "definition_missing", wantSelections: true, wantDefinition: false},
		{name: "decrypt_failed", selected: []string{"Accetto"}, storeDef: true, badSecret: true, wantStatus: "decrypt_failed", wantSelections: false, wantDefinition: true},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			chat := types.NewJID("120363000000", types.GroupServer)
			creator := types.NewJID("628111", types.DefaultUserServer)
			voter := types.NewJID("628222", types.DefaultUserServer)
			pollID := types.MessageID("POLL-CONTRACT-1")
			options := []string{"Accetto", "Rifiuto"}

			sender := newPollCryptoClient(t, "poll-contract-sender-"+tc.name, voter)
			require.NoError(t, sender.Store.MsgSecrets.PutMessageSecret(ctx, chat, creator, pollID, bytes.Repeat([]byte{0x42}, 32)))
			voteMessage, err := sender.BuildPollVote(ctx, &types.MessageInfo{
				MessageSource: types.MessageSource{Chat: chat, Sender: creator, IsGroup: true},
				ID:            pollID,
			}, tc.selected)
			require.NoError(t, err)

			receiver := sender
			if tc.badSecret {
				receiver = newPollCryptoClient(t, "poll-contract-receiver-bad-"+tc.name, voter)
				require.NoError(t, receiver.Store.MsgSecrets.PutMessageSecret(ctx, chat, creator, pollID, bytes.Repeat([]byte{0x99}, 32)))
			}

			store := newMemoryPollStore()
			if tc.storeDef {
				defOptions := make([]domainChatStorage.PollOption, 0, len(options))
				for _, name := range options {
					defOptions = append(defOptions, domainChatStorage.PollOption{Name: name, Hash: pollOptionHash(name)})
				}
				require.NoError(t, store.UpsertPollDefinition(&domainChatStorage.PollDefinition{
					DeviceID: voter.String(), ChatJID: chat.String(), PollMessageID: string(pollID),
					Question: "Condizioni di servizio", Options: defOptions, SelectableOptionCount: 1, Version: "v1",
				}))
			}

			evt := &events.Message{
				Info: types.MessageInfo{
					MessageSource: types.MessageSource{Chat: chat, Sender: voter, IsGroup: true, IsFromMe: true},
					ID:            types.MessageID("VOTE-CONTRACT-" + string(rune('A'+i))),
					Timestamp:     time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC),
				},
				Message: voteMessage,
			}
			poll := preparePollWebhookPayload(ctx, receiver, store, evt)
			require.NotNil(t, poll)
			webhookEvent, err := createWebhookEvent(ctx, receiver, evt, poll)
			require.NoError(t, err)
			envelope := map[string]any{"event": webhookEvent.Event, "device_id": webhookEvent.DeviceID, "payload": webhookEvent.Payload}
			encoded, err := json.MarshalIndent(envelope, "", "  ")
			require.NoError(t, err)
			encoded = append(encoded, '\n')

			fixture := filepath.Join(pollContractFixtureDir, "vote_"+tc.name+".json")
			if os.Getenv("UPDATE_POLL_CONTRACT_FIXTURES") == "1" {
				require.NoError(t, os.MkdirAll(pollContractFixtureDir, 0o755))
				require.NoError(t, os.WriteFile(fixture, encoded, 0o644))
			}
			golden, err := os.ReadFile(fixture)
			require.NoError(t, err, "missing golden fixture; run with UPDATE_POLL_CONTRACT_FIXTURES=1")
			require.JSONEq(t, string(golden), string(encoded), "webhook JSON drifted from golden fixture %s", fixture)

			assertOrchestratorPollContract(t, tc, encoded)
		})
	}
}

// assertOrchestratorPollContract mirrors, key by key, what leo-orchestrator
// PR #808 reads (gowa_adapter.py:1485-1493, legal/polls.py:157-213).
func assertOrchestratorPollContract(t *testing.T, tc pollContractCase, encoded []byte) {
	t.Helper()
	var envelope map[string]any
	require.NoError(t, json.Unmarshal(encoded, &envelope))
	require.Equal(t, "message", envelope["event"])
	payload, ok := envelope["payload"].(map[string]any)
	require.True(t, ok, "payload must be an object")

	// gowa_adapter: vote timestamp comes from the message payload, parsed with
	// datetime.fromisoformat(ts.replace("Z", "+00:00")) and must be tz-aware.
	ts, ok := payload["timestamp"].(string)
	require.True(t, ok)
	parsed, err := time.Parse(time.RFC3339, ts)
	require.NoError(t, err)
	require.Equal(t, time.UTC, parsed.Location())

	// gowa_adapter: payload["poll"] must be a dict to set gowa_poll_event;
	// it also makes _is_user_text false (non-empty key outside the text set).
	poll, ok := payload["poll"].(map[string]any)
	require.True(t, ok, "payload.poll must be an object")
	require.Equal(t, "vote", poll["type"])
	require.Equal(t, "POLL-CONTRACT-1", poll["poll_id"])
	require.Equal(t, tc.wantStatus, poll["resolution_status"])
	body, _ := payload["body"].(string)
	require.NotEmpty(t, body)

	names, hasNames := poll["selected_options"]
	hashes, hasHashes := poll["selected_option_hashes"]
	require.Equal(t, tc.wantSelections, hasNames, "selected_options presence")
	require.Equal(t, tc.wantSelections, hasHashes, "selected_option_hashes presence")
	if tc.wantSelections {
		nameList, ok := names.([]any)
		require.True(t, ok, "selected_options must be a JSON list (never null)")
		hashList, ok := hashes.([]any)
		require.True(t, ok, "selected_option_hashes must be a JSON list (never null)")
		require.Len(t, hashList, len(nameList)+countUnresolved(tc))
		require.Len(t, hashList, len(tc.selected))
		for i, h := range hashList {
			require.Equal(t, sha256Hex(tc.selected[i]), h, "selected hash = lowercase sha256 hex of the option label")
		}
		if tc.wantStatus == "resolved" {
			require.Equal(t, len(nameList), len(hashList))
			for i, n := range nameList {
				require.Equal(t, tc.selected[i], n)
			}
		}
	}

	_, hasQuestion := poll["question"]
	_, hasOptions := poll["options"]
	require.Equal(t, tc.wantDefinition, hasQuestion, "question presence")
	require.Equal(t, tc.wantDefinition, hasOptions, "options presence")
	if tc.wantDefinition {
		require.Equal(t, "Condizioni di servizio", poll["question"])
		options, ok := poll["options"].([]any)
		require.True(t, ok)
		require.Len(t, options, 2)
		for _, raw := range options {
			option, ok := raw.(map[string]any)
			require.True(t, ok, "options[] items must be objects")
			name, _ := option["name"].(string)
			require.NotEmpty(t, name)
			require.Equal(t, sha256Hex(name), option["hash"], "options[].hash = hashlib.sha256(label.encode()).hexdigest()")
		}
	}
}

// countUnresolved: with definition_missing every selected hash is unmatched,
// so selected_options is empty while selected_option_hashes keeps the hashes.
func countUnresolved(tc pollContractCase) int {
	if tc.wantStatus == "definition_missing" {
		return len(tc.selected)
	}
	return 0
}
