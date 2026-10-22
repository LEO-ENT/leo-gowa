package chatstorage

import (
	"database/sql"
	"os"
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
	require.Len(t, migrations, 53)
	require.Contains(t, migrations[43], "managed_webhook_spool")
	require.Contains(t, migrations[44], "idx_managed_webhook_spool_delivery")
	require.Contains(t, migrations[45], "idx_managed_webhook_spool_due")
	require.Contains(t, migrations[46], "provider_message_receipts")
	require.Contains(t, migrations[47], "CREATE TABLE IF NOT EXISTS poll_definitions")
	for i, m := range migrations[:47] {
		require.NotContains(t, m, "poll_definitions", "migration %d must not touch poll_definitions", i+1)
	}
}

// A database left at fork version 47 must gain poll definitions and all new
// migrations, keeping versions 1..53 contiguous.
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
	require.Equal(t, 53, maxVersion)
	require.Equal(t, 53, rows)
	require.Equal(t, 1, minVersion)
	var ddl string
	require.NoError(t, db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='poll_definitions'`).Scan(&ddl))
	require.True(t, strings.Contains(ddl, "PRIMARY KEY (device_id, chat_jid, poll_message_id)"))

	// Idempotent on restart.
	require.NoError(t, repo.InitializeSchema())
	require.NoError(t, db.QueryRow(`SELECT MAX(version), COUNT(*) FROM schema_info`).Scan(&maxVersion, &rows))
	require.Equal(t, 53, maxVersion)
	require.Equal(t, 53, rows)
}

// The fixture is the actual historical fork schema, independent of the new migration list.
func TestUpgradeHistoricalFork48PreservesData(t *testing.T) {
	db, err := sql.Open(sqlite.DriverName, filepath.Join(t.TempDir(), "fork48.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	fixture, err := os.ReadFile("testdata/fork_v48.sql")
	require.NoError(t, err)
	for _, statement := range strings.Split(string(fixture), ";") {
		if strings.TrimSpace(statement) != "" {
			_, err = db.Exec(statement)
			require.NoError(t, err)
		}
	}
	_, err = db.Exec(`INSERT INTO messages (id, chat_jid, device_id, sender, content, timestamp) VALUES ('message-kept', '1234567890@s.whatsapp.net', 'fake-device', '1234567890@s.whatsapp.net', 'preserve me', CURRENT_TIMESTAMP)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO poll_definitions (device_id, chat_jid, poll_message_id, question, options_json) VALUES ('fake-device', '1234567890@s.whatsapp.net', 'poll-kept', 'Preserve?', '[]')`)
	require.NoError(t, err)
	repo := &SQLiteRepository{db: db}
	version, err := repo.getSchemaVersion()
	require.NoError(t, err)
	require.Equal(t, 48, version)
	require.NoError(t, repo.InitializeSchema())
	require.NoError(t, repo.InitializeSchema(), "restart must not replay ALTER TABLE")
	version, err = repo.getSchemaVersion()
	require.NoError(t, err)
	require.Equal(t, 53, version)
	var content, question, contextMetadata string
	require.NoError(t, db.QueryRow(`SELECT content, context_metadata FROM messages WHERE id='message-kept' AND device_id='fake-device'`).Scan(&content, &contextMetadata))
	require.Equal(t, "preserve me", content)
	require.Equal(t, "", contextMetadata)
	require.NoError(t, db.QueryRow(`SELECT question FROM poll_definitions WHERE poll_message_id='poll-kept' AND device_id='fake-device'`).Scan(&question))
	require.Equal(t, "Preserve?", question)
	for _, name := range []string{"scheduled_sends", "idx_messages_chat_device_timestamp", "idx_scheduled_sends_due", "idx_scheduled_sends_device", "managed_webhook_spool", "provider_message_receipts", "message_edits"} {
		var count int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name=?`, name).Scan(&count))
		require.Equal(t, 1, count, name)
	}
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM schema_info`).Scan(&count))
	require.Equal(t, 53, count)
}
