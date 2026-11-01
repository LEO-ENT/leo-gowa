package usecase

import (
	"context"
	"sort"
	"strings"

	domainUser "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/user"
	"go.mau.fi/whatsmeow/types"
)

// contactLister / pnLIDMapper are the two read paths of the whatsmeow device store
// this listing needs (store.ContactStore / store.LIDStore satisfy them).
type contactLister interface {
	GetAllContacts(ctx context.Context) (map[types.JID]types.ContactInfo, error)
}

type pnLIDMapper interface {
	GetLIDForPN(ctx context.Context, pn types.JID) (types.JID, error)
}

// buildDetailedContacts returns one row per person contact, every name in its own
// field. Decision tree per store key:
//  1. server is neither s.whatsapp.net nor lid -> skipped (group, broadcast, newsletter, bot);
//  2. jid = the key itself (ToNonAD); a PN is NEVER derived from a LID here;
//  3. PN key and the LID store knows its LID -> lid set; lookup error/unknown -> lid omitted;
//  4. names: each kept only when not blank; no precedence, no merging.
//
// A contact store error is returned, never an empty list.
func buildDetailedContacts(ctx context.Context, contacts contactLister, lids pnLIDMapper) ([]domainUser.MyListContactsDetailedData, error) {
	all, err := contacts.GetAllContacts(ctx)
	if err != nil {
		return nil, err
	}
	rows := make([]domainUser.MyListContactsDetailedData, 0, len(all))
	for key, info := range all {
		jid := key.ToNonAD()
		if jid.Server != types.DefaultUserServer && jid.Server != types.HiddenUserServer {
			continue
		}
		row := domainUser.MyListContactsDetailedData{
			JID:          jid.String(),
			FullName:     keepName(info.FullName),
			FirstName:    keepName(info.FirstName),
			PushName:     keepName(info.PushName),
			BusinessName: keepName(info.BusinessName),
		}
		if jid.Server == types.DefaultUserServer && lids != nil {
			if lid, lookupErr := lids.GetLIDForPN(ctx, jid); lookupErr == nil && !lid.IsEmpty() {
				row.LID = lid.ToNonAD().String()
			}
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].JID < rows[j].JID })
	return rows, nil
}

func keepName(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return value
}
