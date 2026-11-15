package whatsapp

// LEO decision 2026-11-15: device.* alerts fall back to the global webhook when the
// per-device webhook config cannot be resolved; every other event stays fail-closed
// (patch 86345a6).

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
	"github.com/stretchr/testify/require"
)

const alertTestDeviceJID = "393330000009@s.whatsapp.net"

type alertDelivery struct {
	url string
	cfg *domainChatStorage.DeviceWebhookConfig
}

func captureAlertDeliveries(t *testing.T) <-chan alertDelivery {
	t.Helper()
	captureDeviceWebhooks(t) // restores globals; submit stub replaced below
	got := make(chan alertDelivery, 4)
	submitWebhookFn = func(_ context.Context, payload map[string]any, url string, cfg *domainChatStorage.DeviceWebhookConfig) error {
		// The stub is package-global: ignore late asynchronous deliveries of other tests.
		if payload["device_id"] != alertTestDeviceJID {
			return nil
		}
		got <- alertDelivery{url: url, cfg: cfg}
		return nil
	}
	return got
}

func noAlertDelivery(t *testing.T, got <-chan alertDelivery) {
	t.Helper()
	select {
	case d := <-got:
		t.Fatalf("unexpected delivery to %s", d.url)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestDeviceAlertFallsBackToGlobalWhenDeviceConfigFails(t *testing.T) {
	for _, event := range []string{EventTypeDeviceTemporaryBan, EventTypeDeviceLoggedOut, EventTypeDeviceReachoutTimelock} {
		t.Run(event, func(t *testing.T) {
			got := captureAlertDeliveries(t)
			body := map[string]any{"event": event, "device_id": alertTestDeviceJID, "payload": map[string]any{"code": 102}}
			err := forwardPayloadWithResolvedWebhookConfig(context.Background(), body, event, nil, errDeviceWebhookConfigUnavailable)
			require.NoError(t, err)
			d := recvAlert(t, got)
			require.Equal(t, "https://global.example.test/webhook", d.url)
			require.Nil(t, d.cfg, "global leg must sign with the global secret")
		})
	}
}

func TestMessageStaysFailClosedWhenDeviceConfigFails(t *testing.T) {
	got := captureAlertDeliveries(t)
	lookupErr := errors.New("device record lookup failed")
	webhookStorageForTest = func(string) (*domainChatStorage.DeviceRecord, error) { return nil, lookupErr }
	body := map[string]any{"event": EventTypeMessage, "device_id": alertTestDeviceJID, "payload": map[string]any{"id": "M1", "body": "ciao"}}
	err := forwardPayloadToConfiguredWebhooks(context.Background(), body, EventTypeMessage)
	require.Error(t, err)
	noAlertDelivery(t, got)

	err = forwardPayloadWithResolvedWebhookConfig(context.Background(), body, EventTypeMessage, nil, errDeviceWebhookConfigUnavailable)
	require.ErrorIs(t, err, errDeviceWebhookConfigUnavailable)
	noAlertDelivery(t, got)
}

func TestDeviceAlertUsesDeviceConfigWhenResolved(t *testing.T) {
	got := captureAlertDeliveries(t)
	url := "https://device.example.test/hook"
	cfg := &domainChatStorage.DeviceWebhookConfig{WebhookURL: &url, WebhookSecret: "device-secret"}
	body := map[string]any{"event": EventTypeDeviceTemporaryBan, "device_id": alertTestDeviceJID, "payload": map[string]any{"code": 102}}
	require.NoError(t, forwardPayloadWithResolvedWebhookConfig(context.Background(), body, EventTypeDeviceTemporaryBan, cfg, nil))
	d := recvAlert(t, got)
	require.Equal(t, url, d.url)
	require.Same(t, cfg, d.cfg)
	noAlertDelivery(t, got)
}

func TestDeviceAlertWithoutGlobalWebhookReturnsResolveError(t *testing.T) {
	got := captureAlertDeliveries(t)
	config.WhatsappWebhook = nil // restored by captureDeviceWebhooks
	body := map[string]any{"event": EventTypeDeviceLoggedOut, "device_id": alertTestDeviceJID, "payload": map[string]any{"code": 401}}
	err := forwardPayloadWithResolvedWebhookConfig(context.Background(), body, EventTypeDeviceLoggedOut, nil, errDeviceWebhookConfigUnavailable)
	require.ErrorIs(t, err, errDeviceWebhookConfigUnavailable)
	noAlertDelivery(t, got)
}

func recvAlert(t *testing.T, got <-chan alertDelivery) alertDelivery {
	t.Helper()
	select {
	case d := <-got:
		return d
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for webhook submission")
		return alertDelivery{}
	}
}
