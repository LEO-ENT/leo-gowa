package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	domainUser "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/user"
	"go.mau.fi/whatsmeow/types"
)

func TestDetailedContactJSONOmitsEmptyOptionalFields(t *testing.T) {
	raw, err := json.Marshal(domainUser.MyListContactsDetailedResponse{
		Data: []domainUser.MyListContactsDetailedData{{JID: "628111@s.whatsapp.net", PushName: "Push"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"data":[{"jid":"628111@s.whatsapp.net","push_name":"Push"}]}`
	if string(raw) != want {
		t.Fatalf("json = %s, want %s", raw, want)
	}
}

type detailedLIDStub struct {
	lidForPN map[types.JID]types.JID
	err      error
}

func (s *detailedLIDStub) GetPNForLID(context.Context, types.JID) (types.JID, error) {
	return types.EmptyJID, s.err
}

func (s *detailedLIDStub) GetLIDForPN(_ context.Context, pn types.JID) (types.JID, error) {
	if s.err != nil {
		return types.EmptyJID, s.err
	}
	return s.lidForPN[pn.ToNonAD()], nil
}

type failingContactLister struct{}

func (failingContactLister) GetAllContacts(context.Context) (map[types.JID]types.ContactInfo, error) {
	return nil, errors.New("store down")
}

func TestBuildDetailedContactsKeepsEveryNameSeparate(t *testing.T) {
	pn := types.NewJID("628111000001", types.DefaultUserServer)
	lidOfPN := types.NewJID("100000000001", types.HiddenUserServer)
	lidOnly := types.NewJID("100000000002", types.HiddenUserServer)
	group := types.NewJID("120363000000000001", types.GroupServer)
	contacts := &chatContactStoreStub{contacts: map[types.JID]types.ContactInfo{
		pn:      {Found: true, FullName: "Marielly Test", FirstName: "Marielly", PushName: "Lili ✿", BusinessName: ""},
		lidOnly: {Found: true, PushName: "Mariella"},
		group:   {Found: true, FullName: "nós-test"},
	}}
	got, err := buildDetailedContacts(context.Background(), contacts, &detailedLIDStub{lidForPN: map[types.JID]types.JID{pn: lidOfPN}})
	if err != nil {
		t.Fatal(err)
	}
	want := []domainUser.MyListContactsDetailedData{
		{JID: lidOnly.String(), PushName: "Mariella"},
		{JID: pn.String(), LID: lidOfPN.String(), FullName: "Marielly Test", FirstName: "Marielly", PushName: "Lili ✿"},
	}
	if len(got) != len(want) {
		t.Fatalf("rows = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("row %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestBuildDetailedContactsDropsBlankNamesAndReturnsEmptyArray(t *testing.T) {
	pn := types.NewJID("628111000003", types.DefaultUserServer)
	got, err := buildDetailedContacts(context.Background(),
		&chatContactStoreStub{contacts: map[types.JID]types.ContactInfo{pn: {FullName: "   ", PushName: "\t"}}},
		&detailedLIDStub{})
	if err != nil || len(got) != 1 || got[0] != (domainUser.MyListContactsDetailedData{JID: pn.String()}) {
		t.Fatalf("got %+v err %v", got, err)
	}
	empty, err := buildDetailedContacts(context.Background(), &chatContactStoreStub{contacts: nil}, nil)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty store must give a non-nil empty slice, got %#v err %v", empty, err)
	}
}

func TestBuildDetailedContactsLIDLookupFailureOmitsOnlyTheLID(t *testing.T) {
	pn := types.NewJID("628111000004", types.DefaultUserServer)
	got, err := buildDetailedContacts(context.Background(),
		&chatContactStoreStub{contacts: map[types.JID]types.ContactInfo{pn: {PushName: "P"}}},
		&detailedLIDStub{err: errors.New("lid store down")})
	if err != nil || len(got) != 1 || got[0].LID != "" || got[0].PushName != "P" {
		t.Fatalf("got %+v err %v", got, err)
	}
}

func TestBuildDetailedContactsPropagatesContactStoreError(t *testing.T) {
	if _, err := buildDetailedContacts(context.Background(), failingContactLister{}, nil); err == nil {
		t.Fatal("a contact store failure must not look like an empty address book")
	}
}
