package rest

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

func TestAppInfoReportsUpstreamUpgradeVersionWithoutDevice(t *testing.T) {
	app := fiber.New()
	InitRestAppInfo(app)
	response, err := app.Test(httptest.NewRequest("GET", "/app/info", nil))
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, 200, response.StatusCode)
	var envelope struct {
		Results struct {
			Version string `json:"version"`
		} `json:"results"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&envelope))
	require.Equal(t, "v9.6.0", envelope.Results.Version)
}
