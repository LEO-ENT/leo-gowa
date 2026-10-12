-- Historical LEO schema from origin/main 5e42fc5 (48 positional migrations).
CREATE TABLE IF NOT EXISTS chats (
			jid VARCHAR(255) NOT NULL,
			device_id VARCHAR(255) NOT NULL DEFAULT '',
			name VARCHAR(255) NOT NULL,
			last_message_time TIMESTAMP NOT NULL,
			ephemeral_expiration INTEGER DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (jid, device_id)
		);
CREATE TABLE IF NOT EXISTS messages (
			id VARCHAR(255) NOT NULL,
			chat_jid VARCHAR(255) NOT NULL,
			device_id VARCHAR(255) NOT NULL DEFAULT '',
			sender VARCHAR(255) NOT NULL,
			content TEXT,
			timestamp TIMESTAMP NOT NULL,
			is_from_me BOOLEAN DEFAULT FALSE,
			media_type VARCHAR(50),
			filename VARCHAR(255),
			url TEXT,
			media_key BLOB,
			file_sha256 BLOB,
			file_enc_sha256 BLOB,
			file_length INTEGER DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (id, chat_jid, device_id)
		);
CREATE TABLE IF NOT EXISTS devices (
			device_id VARCHAR(255) PRIMARY KEY,
			display_name VARCHAR(255) DEFAULT '',
			jid VARCHAR(255) DEFAULT '',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
CREATE INDEX IF NOT EXISTS idx_messages_chat_jid ON messages(chat_jid);
CREATE INDEX IF NOT EXISTS idx_messages_device ON messages(device_id);
CREATE INDEX IF NOT EXISTS idx_messages_timestamp ON messages(timestamp);
CREATE INDEX IF NOT EXISTS idx_messages_media_type ON messages(media_type);
CREATE INDEX IF NOT EXISTS idx_messages_sender ON messages(sender);
CREATE INDEX IF NOT EXISTS idx_chats_last_message ON chats(last_message_time);
CREATE INDEX IF NOT EXISTS idx_chats_name ON chats(name);
CREATE INDEX IF NOT EXISTS idx_chats_device ON chats(device_id);
CREATE INDEX IF NOT EXISTS idx_devices_created_at ON devices(created_at);
ALTER TABLE chats ADD COLUMN archived BOOLEAN DEFAULT FALSE;;
CREATE INDEX IF NOT EXISTS idx_chats_archived ON chats(archived);
ALTER TABLE messages ADD COLUMN call_metadata TEXT DEFAULT '';
ALTER TABLE messages ADD COLUMN referral_metadata TEXT DEFAULT '';
CREATE TABLE IF NOT EXISTS message_reactions (
			message_id VARCHAR(255) NOT NULL,
			chat_jid VARCHAR(255) NOT NULL,
			device_id VARCHAR(255) NOT NULL DEFAULT '',
			reactor_jid VARCHAR(255) NOT NULL,
			emoji TEXT NOT NULL DEFAULT '',
			is_from_me BOOLEAN DEFAULT FALSE,
			reaction_timestamp TIMESTAMP NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (message_id, reactor_jid, device_id)
		);
CREATE INDEX IF NOT EXISTS idx_message_reactions_lookup ON message_reactions(device_id, chat_jid, message_id);
CREATE TABLE IF NOT EXISTS message_edits (
			original_message_id VARCHAR(255) NOT NULL,
			edit_event_id VARCHAR(255) NOT NULL,
			chat_jid VARCHAR(255) NOT NULL,
			device_id VARCHAR(255) NOT NULL DEFAULT '',
			editor VARCHAR(255) DEFAULT '',
			previous_content TEXT DEFAULT '',
			new_content TEXT DEFAULT '',
			edited_at TIMESTAMP NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (original_message_id, edit_event_id, device_id),
			FOREIGN KEY (original_message_id, chat_jid, device_id) REFERENCES messages(id, chat_jid, device_id) ON DELETE CASCADE
		);
CREATE INDEX IF NOT EXISTS idx_message_edits_original ON message_edits(original_message_id);
CREATE INDEX IF NOT EXISTS idx_message_edits_device ON message_edits(device_id);
CREATE INDEX IF NOT EXISTS idx_message_edits_edited_at ON message_edits(edited_at);
CREATE TABLE IF NOT EXISTS chatwoot_message_links (
			device_id VARCHAR(255) NOT NULL DEFAULT '',
			wa_message_id VARCHAR(255) NOT NULL,
			wa_chat_jid VARCHAR(255) NOT NULL DEFAULT '',
			chatwoot_message_id INTEGER NOT NULL DEFAULT 0,
			chatwoot_conversation_id INTEGER NOT NULL DEFAULT 0,
			chatwoot_inbox_id INTEGER NOT NULL DEFAULT 0,
			chatwoot_contact_inbox_source_id VARCHAR(255) NOT NULL DEFAULT '',
			source_id VARCHAR(255) NOT NULL DEFAULT '',
			direction VARCHAR(20) NOT NULL DEFAULT '',
			is_read BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (device_id, wa_message_id)
		);
CREATE INDEX IF NOT EXISTS idx_chatwoot_links_chatwoot_id ON chatwoot_message_links(device_id, chatwoot_message_id);
CREATE INDEX IF NOT EXISTS idx_chatwoot_links_source_id ON chatwoot_message_links(device_id, source_id);
CREATE INDEX IF NOT EXISTS idx_chatwoot_links_conversation ON chatwoot_message_links(chatwoot_conversation_id, updated_at);
CREATE INDEX IF NOT EXISTS idx_chatwoot_links_unread ON chatwoot_message_links(device_id, wa_chat_jid, direction, is_read, updated_at, created_at);
CREATE TABLE IF NOT EXISTS chatwoot_forward_queue (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id VARCHAR(255) NOT NULL DEFAULT '',
			event_name VARCHAR(80) NOT NULL DEFAULT '',
			wa_message_id VARCHAR(255) NOT NULL DEFAULT '',
			payload_json TEXT NOT NULL DEFAULT '',
			attempts INTEGER NOT NULL DEFAULT 0,
			last_error TEXT NOT NULL DEFAULT '',
			next_attempt_at TIMESTAMP NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(device_id, event_name, wa_message_id)
		);
CREATE INDEX IF NOT EXISTS idx_chatwoot_forward_queue_due ON chatwoot_forward_queue(next_attempt_at, id);
ALTER TABLE messages ADD COLUMN direct_path TEXT DEFAULT '';
ALTER TABLE devices ADD COLUMN webhook_url TEXT DEFAULT NULL;
ALTER TABLE devices ADD COLUMN webhook_secret TEXT DEFAULT '';
ALTER TABLE devices ADD COLUMN webhook_events TEXT DEFAULT '';
ALTER TABLE devices ADD COLUMN webhook_insecure_skip_verify BOOLEAN DEFAULT FALSE;
ALTER TABLE devices ADD COLUMN ad_jid VARCHAR(255) DEFAULT '';
CREATE TABLE IF NOT EXISTS chatwoot_device_configs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id VARCHAR(255) NOT NULL DEFAULT '',
			device_jid VARCHAR(255) NOT NULL DEFAULT '',
			chatwoot_url VARCHAR(512) NOT NULL DEFAULT '',
			account_id INTEGER NOT NULL DEFAULT 0,
			inbox_id INTEGER NOT NULL DEFAULT 0,
			api_token TEXT NOT NULL DEFAULT '',
			enabled BOOLEAN NOT NULL DEFAULT 1,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
CREATE UNIQUE INDEX IF NOT EXISTS idx_chatwoot_device_configs_device ON chatwoot_device_configs(device_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_chatwoot_device_configs_jid ON chatwoot_device_configs(device_jid) WHERE device_jid <> '';
CREATE UNIQUE INDEX IF NOT EXISTS idx_chatwoot_device_configs_inbox ON chatwoot_device_configs(chatwoot_url, account_id, inbox_id);
ALTER TABLE chatwoot_message_links ADD COLUMN chatwoot_config_id INTEGER NOT NULL DEFAULT 0;
ALTER TABLE chatwoot_message_links ADD COLUMN chatwoot_account_id INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_chatwoot_links_conversation_account ON chatwoot_message_links(chatwoot_conversation_id, chatwoot_account_id, updated_at);
CREATE INDEX IF NOT EXISTS idx_chatwoot_links_config ON chatwoot_message_links(chatwoot_config_id);
CREATE TABLE IF NOT EXISTS managed_webhook_spool (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			delivery_id VARCHAR(96) NOT NULL,
			device_digest VARCHAR(128) NOT NULL,
			source_session_digest VARCHAR(128) NOT NULL,
			event_name VARCHAR(80) NOT NULL,
			message_id_digest VARCHAR(128) NOT NULL,
			body_hash VARCHAR(128) NOT NULL,
			payload_ciphertext BLOB NULL,
			payload_key_version VARCHAR(80) NOT NULL,
			secret_version VARCHAR(80) NOT NULL,
			status VARCHAR(16) NOT NULL DEFAULT 'pending'
				CHECK (status IN ('pending', 'processing', 'retry', 'completed', 'dead')),
			attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
			max_attempts INTEGER NOT NULL CHECK (max_attempts > 0),
			next_attempt_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			deadline_at TIMESTAMP NOT NULL,
			owner VARCHAR(128) NOT NULL DEFAULT '',
			lease_until TIMESTAMP NULL,
			fence_token INTEGER NOT NULL DEFAULT 0 CHECK (fence_token >= 0),
			last_error_code VARCHAR(64) NOT NULL DEFAULT '',
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			completed_at TIMESTAMP NULL,
			dead_at TIMESTAMP NULL,
			payload_purged_at TIMESTAMP NULL,
			CHECK (payload_ciphertext IS NULL OR length(payload_ciphertext) > 0),
			UNIQUE(device_digest, source_session_digest, event_name, message_id_digest, body_hash)
		);
CREATE UNIQUE INDEX IF NOT EXISTS idx_managed_webhook_spool_delivery ON managed_webhook_spool(delivery_id);
CREATE INDEX IF NOT EXISTS idx_managed_webhook_spool_due ON managed_webhook_spool(status, next_attempt_at, lease_until, id);
CREATE TABLE IF NOT EXISTS provider_message_receipts (
			device_id VARCHAR(255) NOT NULL,
			provider_message_id VARCHAR(64) NOT NULL,
			observed_at TIMESTAMP NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (device_id, provider_message_id)
		);
CREATE TABLE IF NOT EXISTS poll_definitions (
			device_id VARCHAR(255) NOT NULL DEFAULT '',
			chat_jid VARCHAR(255) NOT NULL,
			poll_message_id VARCHAR(255) NOT NULL,
			question TEXT NOT NULL DEFAULT '',
			options_json TEXT NOT NULL DEFAULT '[]',
			selectable_option_count INTEGER NOT NULL DEFAULT 0,
			version VARCHAR(16) NOT NULL DEFAULT '',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (device_id, chat_jid, poll_message_id)
		);
CREATE TABLE schema_info (version INTEGER PRIMARY KEY, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
INSERT INTO schema_info(version) VALUES (1);
INSERT INTO schema_info(version) VALUES (2);
INSERT INTO schema_info(version) VALUES (3);
INSERT INTO schema_info(version) VALUES (4);
INSERT INTO schema_info(version) VALUES (5);
INSERT INTO schema_info(version) VALUES (6);
INSERT INTO schema_info(version) VALUES (7);
INSERT INTO schema_info(version) VALUES (8);
INSERT INTO schema_info(version) VALUES (9);
INSERT INTO schema_info(version) VALUES (10);
INSERT INTO schema_info(version) VALUES (11);
INSERT INTO schema_info(version) VALUES (12);
INSERT INTO schema_info(version) VALUES (13);
INSERT INTO schema_info(version) VALUES (14);
INSERT INTO schema_info(version) VALUES (15);
INSERT INTO schema_info(version) VALUES (16);
INSERT INTO schema_info(version) VALUES (17);
INSERT INTO schema_info(version) VALUES (18);
INSERT INTO schema_info(version) VALUES (19);
INSERT INTO schema_info(version) VALUES (20);
INSERT INTO schema_info(version) VALUES (21);
INSERT INTO schema_info(version) VALUES (22);
INSERT INTO schema_info(version) VALUES (23);
INSERT INTO schema_info(version) VALUES (24);
INSERT INTO schema_info(version) VALUES (25);
INSERT INTO schema_info(version) VALUES (26);
INSERT INTO schema_info(version) VALUES (27);
INSERT INTO schema_info(version) VALUES (28);
INSERT INTO schema_info(version) VALUES (29);
INSERT INTO schema_info(version) VALUES (30);
INSERT INTO schema_info(version) VALUES (31);
INSERT INTO schema_info(version) VALUES (32);
INSERT INTO schema_info(version) VALUES (33);
INSERT INTO schema_info(version) VALUES (34);
INSERT INTO schema_info(version) VALUES (35);
INSERT INTO schema_info(version) VALUES (36);
INSERT INTO schema_info(version) VALUES (37);
INSERT INTO schema_info(version) VALUES (38);
INSERT INTO schema_info(version) VALUES (39);
INSERT INTO schema_info(version) VALUES (40);
INSERT INTO schema_info(version) VALUES (41);
INSERT INTO schema_info(version) VALUES (42);
INSERT INTO schema_info(version) VALUES (43);
INSERT INTO schema_info(version) VALUES (44);
INSERT INTO schema_info(version) VALUES (45);
INSERT INTO schema_info(version) VALUES (46);
INSERT INTO schema_info(version) VALUES (47);
INSERT INTO schema_info(version) VALUES (48);
