package usecase

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
	domainSend "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/send"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/chatstorage"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/whatsapp"
	pkgSQLite "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/sqlite"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.mau.fi/whatsmeow/types"
)

type sentPollBusyError struct{}

func (sentPollBusyError) Error() string { return "private-backend-error" }
func (sentPollBusyError) Code() int     { return 5 }

type sentPollRetryRepo struct {
	domainChatStorage.IChatStorageRepository
	calls       int
	busyCalls   int
	terminal    error
	contexts    []context.Context
	definitions []*domainChatStorage.PollDefinition
	onBusy      func()
}

func (r *sentPollRetryRepo) UpsertPollDefinitionContext(ctx context.Context, definition *domainChatStorage.PollDefinition) error {
	r.calls++
	if err := ctx.Err(); err != nil {
		return err
	}
	r.contexts = append(r.contexts, ctx)
	copyDefinition := *definition
	r.definitions = append(r.definitions, &copyDefinition)
	if r.calls <= r.busyCalls {
		if r.onBusy != nil {
			r.onBusy()
		}
		return fmt.Errorf("wrapped: %w", sentPollBusyError{})
	}
	return r.terminal
}

func sentPollRetryFixture(repo domainChatStorage.IChatStorageRepository) (serviceSend, context.Context, types.JID, domainSend.PollRequest) {
	instance := whatsapp.NewDeviceInstance("1234567890@s.whatsapp.net", nil, nil)
	ctx := whatsapp.ContextWithDevice(context.Background(), instance)
	return serviceSend{chatStorageRepo: repo}, ctx, types.NewJID("120363000000000000", types.GroupServer), domainSend.PollRequest{Question: "Pick?", Options: []string{"A", "B"}, MaxAnswer: 1}
}

func TestPersistSentPollDefinitionRetriesTransientBusy(t *testing.T) {
	var logs bytes.Buffer
	previousOutput := logrus.StandardLogger().Out
	logrus.SetOutput(&logs)
	defer logrus.SetOutput(previousOutput)
	repo := &sentPollRetryRepo{busyCalls: 2}
	service, ctx, recipient, request := sentPollRetryFixture(repo)
	timestamp := time.Date(2026, 10, 11, 10, 0, 0, 0, time.UTC)
	require.NoError(t, service.persistSentPollDefinition(ctx, recipient, "POLL-SENT-RETRY", request, timestamp))
	require.Equal(t, 3, repo.calls)
	require.Contains(t, logs.String(), "level=warning")
	require.NotContains(t, logs.String(), "private-backend-error")
	require.NotContains(t, logs.String(), "1234567890")
	require.NotContains(t, logs.String(), "POLL-SENT-RETRY")
	for _, definition := range repo.definitions {
		require.Equal(t, "1234567890@s.whatsapp.net", definition.DeviceID)
		require.Equal(t, timestamp, definition.UpdatedAt)
		require.Equal(t, timestamp, definition.CreatedAt)
		require.Equal(t, repo.definitions[0].Options, definition.Options)
	}
}

func TestPersistSentPollDefinitionStopsAfterExhaustionOrNonBusy(t *testing.T) {
	t.Run("busy exhausted", func(t *testing.T) {
		repo := &sentPollRetryRepo{busyCalls: sentPollPersistenceAttempts}
		service, ctx, recipient, request := sentPollRetryFixture(repo)
		err := service.persistSentPollDefinition(ctx, recipient, "POLL-EXHAUSTED", request)
		require.True(t, pkgSQLite.IsBusy(err))
		require.Equal(t, sentPollPersistenceAttempts, repo.calls)
	})
	t.Run("nonbusy immediate", func(t *testing.T) {
		failure := errors.New("storage unavailable")
		repo := &sentPollRetryRepo{terminal: failure}
		service, ctx, recipient, request := sentPollRetryFixture(repo)
		require.ErrorIs(t, service.persistSentPollDefinition(ctx, recipient, "POLL-NONBUSY", request), failure)
		require.Equal(t, 1, repo.calls)
	})
}

func TestPersistSentPollDefinitionSurvivesHTTPCancellation(t *testing.T) {
	repo := &sentPollRetryRepo{busyCalls: 1}
	service, baseCtx, recipient, request := sentPollRetryFixture(repo)
	ctx, cancel := context.WithCancel(baseCtx)
	repo.onBusy = cancel
	defer cancel()
	require.NoError(t, service.persistSentPollDefinition(ctx, recipient, "POLL-CANCELLED-HTTP", request))
	require.ErrorIs(t, ctx.Err(), context.Canceled)
	require.Equal(t, 2, repo.calls)
	for _, persistCtx := range repo.contexts {
		instance, ok := whatsapp.DeviceFromContext(persistCtx)
		require.True(t, ok)
		require.Equal(t, "1234567890@s.whatsapp.net", instance.ID())
		deadline, ok := persistCtx.Deadline()
		require.True(t, ok)
		require.LessOrEqual(t, time.Until(deadline), sentPollPersistenceTimeout)
	}
}

func TestRetrySentPollDefinitionDeadlineStopsBusyWait(t *testing.T) {
	repo := &sentPollRetryRepo{busyCalls: sentPollPersistenceAttempts}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	err := retrySentPollDefinition(ctx, repo, &domainChatStorage.PollDefinition{})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Equal(t, 1, repo.calls)
}

type sentPollLockedRepo struct {
	domainChatStorage.IChatStorageRepository
	calls         int
	releaseWriter func() error
}

func (r *sentPollLockedRepo) UpsertPollDefinitionContext(ctx context.Context, definition *domainChatStorage.PollDefinition) error {
	r.calls++
	err := r.IChatStorageRepository.(pollDefinitionContextWriter).UpsertPollDefinitionContext(ctx, definition)
	if pkgSQLite.IsBusy(err) && r.releaseWriter != nil {
		release := r.releaseWriter
		r.releaseWriter = nil
		if releaseErr := release(); releaseErr != nil {
			return releaseErr
		}
	}
	return err
}

func TestPersistSentPollDefinitionRetriesRealSQLiteWriterLock(t *testing.T) {
	uri := "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "poll-retry.db"))
	db, err := sql.Open(pkgSQLite.DriverName, uri)
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	defer db.Close()
	repo := chatstorage.NewStorageRepository(db)
	require.NoError(t, repo.InitializeSchema())
	_, err = db.Exec("PRAGMA busy_timeout=0")
	require.NoError(t, err)
	writer, err := sql.Open(pkgSQLite.DriverName, uri)
	require.NoError(t, err)
	writer.SetMaxOpenConns(1)
	defer writer.Close()
	_, err = writer.Exec("BEGIN IMMEDIATE")
	require.NoError(t, err)
	defer writer.Exec("ROLLBACK")
	lockedRepo := &sentPollLockedRepo{IChatStorageRepository: repo, releaseWriter: func() error { _, err := writer.Exec("COMMIT"); return err }}
	service, ctx, recipient, request := sentPollRetryFixture(lockedRepo)
	require.NoError(t, service.persistSentPollDefinition(ctx, recipient, "POLL-SQLITE-LOCK", request))
	require.Equal(t, 2, lockedRepo.calls)
	stored, err := repo.GetPollDefinition("1234567890@s.whatsapp.net", recipient.String(), "POLL-SQLITE-LOCK")
	require.NoError(t, err)
	require.NotNil(t, stored)
	require.Equal(t, "Pick?", stored.Question)
	require.Len(t, stored.Options, 2)
}
