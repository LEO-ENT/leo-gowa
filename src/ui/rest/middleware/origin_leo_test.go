package middleware

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

// LEO patch queue (F3-T4): leo-orchestrator calls GOWA server-to-server with
// httpx, never with Origin, Referer or Sec-Fetch-* headers. The v9.9 browser
// origin guard (#924) must keep accepting those calls on the state-changing
// routes orch uses.
func TestBrowserOriginGuardAcceptsOrchestratorCallsWithoutOrigin(t *testing.T) {
	app := fiber.New()
	app.Use(BrowserOriginGuard())
	ok := func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }
	app.Post("/send/message", ok)
	app.Post("/send/image", ok)
	app.Post("/message/:message_id/read", ok)
	app.Post("/provider/messages/lookup", ok)
	app.Get("/user/info", ok)

	for _, ua := range []string{"python-httpx/0.28.1", ""} {
		for _, tc := range []struct{ method, path string }{
			{fiber.MethodPost, "/send/message"},
			{fiber.MethodPost, "/send/image"},
			{fiber.MethodPost, "/message/3EB0A1B2C3D4E5F6071829/read"},
			{fiber.MethodPost, "/provider/messages/lookup"},
			{fiber.MethodGet, "/user/info"},
		} {
			name := tc.method + " " + tc.path + " ua=" + ua
			t.Run(name, func(t *testing.T) {
				req := httptest.NewRequest(tc.method, "http://gowa:3000"+tc.path, strings.NewReader("{}"))
				req.Host = "gowa:3000"
				req.Header.Set("Content-Type", "application/json")
				req.Header.Del("Origin")
				req.Header.Del("Referer")
				if ua != "" {
					req.Header.Set("User-Agent", ua)
				} else {
					req.Header.Del("User-Agent")
				}
				resp, err := app.Test(req)
				require.NoError(t, err)
				require.Equal(t, fiber.StatusOK, resp.StatusCode)
				require.NoError(t, resp.Body.Close())
			})
		}
	}
}

// Counter-check: the same route rejects a cross-site browser origin, so the
// test above is not passing because the guard is a no-op.
func TestBrowserOriginGuardStillRejectsCrossSiteOrigin(t *testing.T) {
	app := fiber.New()
	app.Use(BrowserOriginGuard())
	app.Post("/send/message", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	req := httptest.NewRequest(fiber.MethodPost, "http://gowa:3000/send/message", strings.NewReader("{}"))
	req.Host = "gowa:3000"
	req.Header.Set("Origin", "https://evil.example")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusForbidden, resp.StatusCode)
	require.NoError(t, resp.Body.Close())
}
