package rest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	domainUser "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/user"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/whatsapp"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/rest/middleware"
	"github.com/gofiber/fiber/v3"
)

type detailedContactsServiceStub struct {
	domainUser.IUserUsecase
	calls  int
	device *whatsapp.DeviceInstance
}

func (s *detailedContactsServiceStub) MyListContactsDetailed(ctx context.Context) (domainUser.MyListContactsDetailedResponse, error) {
	s.calls++
	s.device, _ = whatsapp.DeviceFromContext(ctx)
	return domainUser.MyListContactsDetailedResponse{Data: []domainUser.MyListContactsDetailedData{
		{JID: "628111000001@s.whatsapp.net", FullName: "Marielly Test", PushName: "Lili"},
	}}, nil
}

func detailedApp(device *whatsapp.DeviceInstance, service domainUser.IUserUsecase) *fiber.App {
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error { // stands in for DeviceMiddleware, which may fall back to the default device
		c.Locals("device", device)
		return c.Next()
	})
	InitRestUser(app, service)
	return app
}

func TestDetailedContactsRequiresAnExplicitDevice(t *testing.T) {
	service := &detailedContactsServiceStub{}
	app := detailedApp(whatsapp.NewDeviceInstance("leo-default", nil, nil), service)
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/user/my/contacts/detailed", nil))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadRequest || service.calls != 0 {
		t.Fatalf("status/calls = %d/%d body=%s, want 400/0", resp.StatusCode, service.calls, body)
	}
	var envelope map[string]any
	if err := json.Unmarshal(body, &envelope); err != nil || envelope["code"] != "DEVICE_ID_REQUIRED" {
		t.Fatalf("envelope = %s", body)
	}
}

func TestDetailedContactsServesTheSelectedDevice(t *testing.T) {
	service := &detailedContactsServiceStub{}
	device := whatsapp.NewDeviceInstance("owner-personal", nil, nil)
	app := detailedApp(device, service)
	req := httptest.NewRequest(http.MethodGet, "/user/my/contacts/detailed", nil)
	req.Header.Set(middleware.DeviceIDHeader, "owner-personal")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || service.calls != 1 || service.device != device {
		t.Fatalf("status=%d calls=%d body=%s", resp.StatusCode, service.calls, body)
	}
	var envelope struct {
		Code    string                                    `json:"code"`
		Results domainUser.MyListContactsDetailedResponse `json:"results"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil || envelope.Code != "SUCCESS" ||
		len(envelope.Results.Data) != 1 || envelope.Results.Data[0].FullName != "Marielly Test" {
		t.Fatalf("envelope = %s", body)
	}
}

func TestPlainContactsRouteIsUnchanged(t *testing.T) {
	// /user/my/contacts keeps its compatibility behaviour (no explicit device required).
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals("device", whatsapp.NewDeviceInstance("d", nil, nil))
		return c.Next()
	})
	stub := &plainContactsStub{}
	InitRestUser(app, stub)
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/user/my/contacts", nil))
	if err != nil || resp.StatusCode != http.StatusOK || stub.calls != 1 {
		t.Fatalf("status=%v calls=%d err=%v", resp, stub.calls, err)
	}
}

type plainContactsStub struct {
	domainUser.IUserUsecase
	calls int
}

func (s *plainContactsStub) MyListContacts(context.Context) (domainUser.MyListContactsResponse, error) {
	s.calls++
	return domainUser.MyListContactsResponse{}, nil
}
