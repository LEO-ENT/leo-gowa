package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	domainGroup "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/group"
	pkgError "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/error"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waVnameCert"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
)

type countingContactStore struct {
	store.NoopStore
	contacts map[types.JID]types.ContactInfo
	reads    int
}

func (s *countingContactStore) GetContact(_ context.Context, jid types.JID) (types.ContactInfo, error) {
	s.reads++
	return s.contacts[jid], nil
}

func TestResolveParticipantDisplayNamePreservesSourceOrdering(t *testing.T) {
	participantJID := types.NewJID("628123456789", types.DefaultUserServer)
	verifiedName := "Verified Business"
	userInfo := map[types.JID]types.UserInfo{
		participantJID: {
			VerifiedName: &types.VerifiedName{
				Details: &waVnameCert.VerifiedNameCertificate_Details{
					VerifiedName: &verifiedName,
				},
			},
		},
	}

	tests := []struct {
		name      string
		seed      string
		contact   types.ContactInfo
		want      string
		wantReads int
	}{
		{
			name:      "seed wins before contact and verified name",
			seed:      "Participant Seed",
			contact:   types.ContactInfo{Found: true, FullName: "Saved Contact"},
			want:      "Participant Seed",
			wantReads: 0,
		},
		{
			name:      "contact wins before verified name",
			contact:   types.ContactInfo{Found: true, FullName: "Saved Contact"},
			want:      "Saved Contact",
			wantReads: 1,
		},
		{
			name:      "verified name follows an unavailable contact",
			want:      "Verified Business",
			wantReads: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contacts := &countingContactStore{
				contacts: map[types.JID]types.ContactInfo{
					participantJID: tt.contact,
				},
			}
			client := &whatsmeow.Client{
				Store: &store.Device{Contacts: contacts},
			}

			got := resolveParticipantDisplayName(
				context.Background(),
				client,
				tt.seed,
				participantJID,
				participantJID,
				userInfo,
			)
			if got != tt.want {
				t.Fatalf("resolveParticipantDisplayName() = %q, want %q", got, tt.want)
			}
			if contacts.reads != tt.wantReads {
				t.Fatalf("contact reads = %d, want %d", contacts.reads, tt.wantReads)
			}
		})
	}
}

func TestGroupParticipantCanonicalPNFreshEntryOnly(t *testing.T) {
	// Reserved NANP 555 fixtures: no client, session store or cached LID map is
	// supplied to the helper. Evidence must come from this exact roster entry.
	pn := types.JID{User: "12025550101", Server: types.DefaultUserServer}
	otherPN := types.JID{User: "12025550102", Server: types.DefaultUserServer}
	lid := types.JID{User: "555000001", Server: types.HiddenUserServer}
	otherLID := types.JID{User: "555000002", Server: types.HiddenUserServer}
	withAgent := func(jid types.JID) types.JID { jid.RawAgent = 1; return jid }
	withDevice := func(jid types.JID) types.JID { jid.Device = 1; return jid }
	withIntegrator := func(jid types.JID) types.JID { jid.Integrator = 1; return jid }

	tests := []struct {
		name  string
		entry types.GroupParticipant
		want  string
	}{
		{"phone wire alone", types.GroupParticipant{JID: pn}, pn.String()},
		{"phone wire exact phone field", types.GroupParticipant{JID: pn, PhoneNumber: pn}, pn.String()},
		{"fresh lid and same entry phone", types.GroupParticipant{JID: lid, PhoneNumber: pn}, pn.String()},
		{"fresh lid agrees with lid field", types.GroupParticipant{JID: lid, LID: lid, PhoneNumber: pn}, pn.String()},
		{"failed phone participant is not membership proof", types.GroupParticipant{JID: pn, Error: 403}, ""},
		{"failed lid participant is not membership proof", types.GroupParticipant{JID: lid, PhoneNumber: pn, Error: 403}, ""},
		{"phone pending add request is not membership proof", types.GroupParticipant{JID: pn, AddRequest: &types.GroupParticipantAddRequest{}}, ""},
		{"lid pending add request is not membership proof", types.GroupParticipant{JID: lid, PhoneNumber: pn, AddRequest: &types.GroupParticipantAddRequest{}}, ""},
		{"missing entire entry", types.GroupParticipant{}, ""},
		{"unresolved lid never becomes phone digits", types.GroupParticipant{JID: lid}, ""},
		{"unresolved phone shaped lid never becomes phone", types.GroupParticipant{JID: types.JID{User: pn.User, Server: types.HiddenUserServer}}, ""},
		{"phone field is lid", types.GroupParticipant{JID: lid, PhoneNumber: lid}, ""},
		{"phone wire conflicts with phone field", types.GroupParticipant{JID: pn, PhoneNumber: otherPN}, ""},
		{"phone wire cannot use lid phone field", types.GroupParticipant{JID: pn, PhoneNumber: lid}, ""},
		{"foreign lid field", types.GroupParticipant{JID: lid, LID: otherLID, PhoneNumber: pn}, ""},
		{"phone accidentally used as lid field", types.GroupParticipant{JID: lid, LID: pn, PhoneNumber: pn}, ""},
		{"unknown primary server", types.GroupParticipant{JID: types.JID{User: pn.User, Server: "synthetic.invalid"}, PhoneNumber: pn}, ""},
		{"legacy phone server is not canonical", types.GroupParticipant{JID: types.JID{User: pn.User, Server: "c.us"}}, ""},
		{"unknown phone field server", types.GroupParticipant{JID: lid, PhoneNumber: types.JID{User: pn.User, Server: "synthetic.invalid"}}, ""},
		{"missing primary user", types.GroupParticipant{JID: types.JID{Server: types.DefaultUserServer}, PhoneNumber: pn}, ""},
		{"missing primary server", types.GroupParticipant{JID: types.JID{User: pn.User}, PhoneNumber: pn}, ""},
		{"missing primary cannot borrow phone field", types.GroupParticipant{PhoneNumber: pn}, ""},
		{"missing phone field user", types.GroupParticipant{JID: lid, PhoneNumber: types.JID{Server: types.DefaultUserServer}}, ""},
		{"missing phone field server", types.GroupParticipant{JID: lid, PhoneNumber: types.JID{User: pn.User}}, ""},
		{"partial phone field cannot supplement phone wire", types.GroupParticipant{JID: pn, PhoneNumber: types.JID{User: pn.User}}, ""},
		{"phone wire raw agent", types.GroupParticipant{JID: withAgent(pn)}, ""},
		{"phone wire device", types.GroupParticipant{JID: withDevice(pn)}, ""},
		{"phone wire integrator", types.GroupParticipant{JID: withIntegrator(pn)}, ""},
		{"lid wire raw agent", types.GroupParticipant{JID: withAgent(lid), PhoneNumber: pn}, ""},
		{"lid wire device", types.GroupParticipant{JID: withDevice(lid), PhoneNumber: pn}, ""},
		{"lid wire integrator", types.GroupParticipant{JID: withIntegrator(lid), PhoneNumber: pn}, ""},
		{"fresh phone field raw agent", types.GroupParticipant{JID: lid, PhoneNumber: withAgent(pn)}, ""},
		{"fresh phone field device", types.GroupParticipant{JID: lid, PhoneNumber: withDevice(pn)}, ""},
		{"fresh phone field integrator", types.GroupParticipant{JID: lid, PhoneNumber: withIntegrator(pn)}, ""},
		{"lid field device conflicts with wire", types.GroupParticipant{JID: lid, LID: withDevice(lid), PhoneNumber: pn}, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := groupParticipantCanonicalPN(test.entry); got != test.want {
				t.Errorf("canonical fresh-entry evidence = %q, want %q", got, test.want)
			}
		})
	}
	for _, user := range []string{"", "+12025550101", " 12025550101", "12025550101 ", "1202555.0101", "12025550101:1", "12025550101\n", "synthetic-person", "１２０２５５５０１０１"} {
		t.Run("malformed user "+user, func(t *testing.T) {
			for _, entry := range []types.GroupParticipant{
				{JID: types.JID{User: user, Server: types.DefaultUserServer}},
				{JID: types.JID{User: user, Server: types.HiddenUserServer}, PhoneNumber: pn},
				{JID: lid, PhoneNumber: types.JID{User: user, Server: types.DefaultUserServer}},
			} {
				if got := groupParticipantCanonicalPN(entry); got != "" {
					t.Errorf("malformed fresh-entry user unexpectedly supplied canonical evidence: %q", got)
				}
			}
		})
	}
}

func TestGroupParticipantCanonicalPNJSONPreservesLegacyFields(t *testing.T) {
	pn := types.JID{User: "12025550101", Server: types.DefaultUserServer}
	lid := types.JID{User: "555000001", Server: types.HiddenUserServer}
	tests := []struct {
		name       string
		entry      types.GroupParticipant
		canonical  string
		admin      bool
		superAdmin bool
	}{
		{"canonical phone wire", types.GroupParticipant{JID: pn}, pn.String(), true, false},
		{"fresh lid alias", types.GroupParticipant{JID: lid, LID: lid, PhoneNumber: pn}, pn.String(), false, true},
		{"unverified lid omitted", types.GroupParticipant{JID: lid}, "", false, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			participant := domainGroup.GroupParticipant{
				JID: test.entry.JID.String(), CanonicalPN: groupParticipantCanonicalPN(test.entry),
				PhoneNumber: "12025550101", LID: lid.String(), DisplayName: "Synthetic roster person",
				IsAdmin: test.admin, IsSuperAdmin: test.superAdmin,
			}
			encoded, err := json.Marshal(participant)
			if err != nil {
				t.Fatal(err)
			}
			var payload map[string]any
			if err := json.Unmarshal(encoded, &payload); err != nil {
				t.Fatal(err)
			}
			canonical, present := payload["canonical_pn"]
			if test.canonical == "" {
				if present {
					t.Errorf("unverified canonical_pn must be omitted, got %v", canonical)
				}
			} else if !present || canonical != test.canonical {
				t.Errorf("canonical_pn = %v (present %v), want %q", canonical, present, test.canonical)
			}
			for key, want := range map[string]any{
				"jid": test.entry.JID.String(), "phone_number": "12025550101", "lid": lid.String(),
				"display_name": "Synthetic roster person", "is_admin": test.admin, "is_super_admin": test.superAdmin,
			} {
				if got, ok := payload[key]; !ok || got != want {
					t.Errorf("legacy field %s = %v (present %v), want %v", key, got, ok, want)
				}
			}
		})
	}
}

func TestValidateGroupRosterParticipantsAcceptsFreshPNAndLIDAdmins(t *testing.T) {
	pn := types.JID{User: "12025550101", Server: types.DefaultUserServer}
	otherPN := types.JID{User: "12025550102", Server: types.DefaultUserServer}
	lid := types.JID{User: "555000001", Server: types.HiddenUserServer}
	participants := []types.GroupParticipant{
		{JID: pn, PhoneNumber: pn, IsAdmin: true},
		{JID: lid, LID: lid, PhoneNumber: otherPN, IsAdmin: true, IsSuperAdmin: true},
	}
	if err := validateGroupRosterParticipants(participants); err != nil {
		t.Fatalf("valid fresh administrator roster refused: %v", err)
	}
}

func TestValidateGroupRosterParticipantsRejectsEntireFailedOrPendingRoster(t *testing.T) {
	pn := types.JID{User: "12025550101", Server: types.DefaultUserServer}
	lid := types.JID{User: "555000001", Server: types.HiddenUserServer}
	validAdmin := types.GroupParticipant{
		JID: types.JID{User: "12025550102", Server: types.DefaultUserServer}, IsAdmin: true,
	}
	validSuperAdmin := types.GroupParticipant{
		JID: types.JID{User: "12025550103", Server: types.DefaultUserServer}, IsAdmin: true, IsSuperAdmin: true,
	}
	const privateRequestCode = "synthetic-private-add-request-code"
	tests := []struct {
		name  string
		entry types.GroupParticipant
	}{
		{"failed phone", types.GroupParticipant{JID: pn, Error: 403}},
		{"failed lid", types.GroupParticipant{JID: lid, PhoneNumber: pn, Error: 403}},
		{"pending phone", types.GroupParticipant{JID: pn, AddRequest: &types.GroupParticipantAddRequest{Code: privateRequestCode}}},
		{"pending lid", types.GroupParticipant{JID: lid, PhoneNumber: pn, AddRequest: &types.GroupParticipantAddRequest{Code: privateRequestCode}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, order := range []struct {
				name  string
				index int
			}{{"first", 0}, {"middle", 1}, {"last", 2}} {
				t.Run(order.name, func(t *testing.T) {
					valid := []types.GroupParticipant{validAdmin, validSuperAdmin}
					roster := append([]types.GroupParticipant{}, valid[:order.index]...)
					roster = append(roster, test.entry)
					roster = append(roster, valid[order.index:]...)
					err := validateGroupRosterParticipants(roster)
					if err == nil {
						t.Fatal("failed or pending row must refuse the whole roster despite valid administrators")
					}
					var typed pkgError.GenericError
					if !errors.As(err, &typed) {
						t.Fatalf("expected typed GenericError, got %T", err)
					}
					if typed.StatusCode() != http.StatusInternalServerError || typed.ErrCode() != "INTERNAL_SERVER_ERROR" || typed.Error() != "Group participants could not be verified" {
						t.Fatalf("unexpected public roster refusal: status=%d code=%s message=%q", typed.StatusCode(), typed.ErrCode(), typed.Error())
					}
					for _, private := range []string{pn.String(), pn.User, lid.String(), lid.User, privateRequestCode, validAdmin.JID.User, validSuperAdmin.JID.User} {
						if strings.Contains(typed.Error()+typed.ErrCode(), private) {
							t.Fatal("public roster refusal contains a private identifier")
						}
					}
				})
			}
		})
	}
}
