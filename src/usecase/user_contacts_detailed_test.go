package usecase

import (
	"encoding/json"
	"testing"

	domainUser "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/user"
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
