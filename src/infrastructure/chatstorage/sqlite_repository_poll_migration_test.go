package chatstorage

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/sqlite"
	"github.com/stretchr/testify/require"
)

// Chatstorage migrations are POSITIONAL (version = MAX(schema_info.version),
// then migrations[version:] run). The LEO fork owns 44-47 (managed webhook
// spool x3, provider receipts); the upstream #816 poll table is backported as
// 48. Re-numbering any of these would silently skip DDL on live databases.
func TestPollDefinitionsMigrationIsAppendedAfterForkMigrations(t *testing.T) {
	migrations := (&SQLiteRepository{}).getMigrations()
	require.Len(t, migrations, 48)
	require.Contains(t, migrations[43], "managed_webhook_spool")
	require.Contains(t, migrations[44], "idx_managed_webhook_spool_delivery")
	require.Contains(t, migrations[45], "idx_managed_webhook_spool_due")
	require.Contains(t, migrations[46], "provider_message_receipts")
	require.Contains(t, migrations[47], "CREATE TABLE IF NOT EXISTS poll_definitions")
	for i, m := range migrations[:47] {
		require.NotContains(t, m, "poll_definitions", "migration %d must not touch poll_definitions", i+1)
	}
}

// A database left at the fork's previous head (version 47) must gain exactly
// migration 48 and keep versions 1..48 contiguous.
func TestPollDefinitionsMigrationUpgradesVersion47Database(t *testing.T) {
	db, err := sql.Open(sqlite.DriverName, filepath.Join(t.TempDir(), "chatstorage.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &SQLiteRepository{db: db}

	_, err = repo.getSchemaVersion()
	require.NoError(t, err)
	migrations := repo.getMigrations()
	for i := 0; i < 47; i++ {
		require.NoError(t, repo.runMigration(migrations[i], i+1), "migration %d", i+1)
	}
	var present int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='poll_definitions'`).Scan(&present))
	require.Zero(t, present)

	require.NoError(t, repo.InitializeSchema())

	var maxVersion, rows, minVersion int
	require.NoError(t, db.QueryRow(`SELECT MAX(version), COUNT(*), MIN(version) FROM schema_info`).Scan(&maxVersion, &rows, &minVersion))
	require.Equal(t, 48, maxVersion)
	require.Equal(t, 48, rows)
	require.Equal(t, 1, minVersion)
	var ddl string
	require.NoError(t, db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='poll_definitions'`).Scan(&ddl))
	require.True(t, strings.Contains(ddl, "PRIMARY KEY (device_id, chat_jid, poll_message_id)"))

	// Idempotent on restart.
	require.NoError(t, repo.InitializeSchema())
	require.NoError(t, db.QueryRow(`SELECT MAX(version), COUNT(*) FROM schema_info`).Scan(&maxVersion, &rows))
	require.Equal(t, 48, maxVersion)
	require.Equal(t, 48, rows)
}
