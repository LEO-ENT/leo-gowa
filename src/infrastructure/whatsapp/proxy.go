package whatsapp

import (
	"net/url"
	"strings"

	"go.mau.fi/whatsmeow"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// LEO typed PII boundary (6e84a1e): proxy configuration logs only fixed event
// names, never the URL, scheme, host or device ID. redactProxyURL is kept from
// upstream #926 for its own tests and callers.
const (
	proxyConfiguredEvent     = "whatsapp_proxy.configured"
	proxyConfigurationFailed = "whatsapp_proxy.configuration_failed"
)

func configureOutboundProxy(client *whatsmeow.Client, rawURL string, logger waLog.Logger) {
	if rawURL == "" {
		return
	}
	logProxyConfigurationResult(logger, client.SetProxyAddress(rawURL))
}

func logProxyConfigurationResult(logger waLog.Logger, err error) {
	if err != nil {
		logger.Errorf(proxyConfigurationFailed)
		return
	}
	logger.Infof(proxyConfiguredEvent)
}

// redactProxyURL returns a form of the proxy URL safe for logging: the
// userinfo (and any unparseable URL entirely) is masked so credentials from
// WHATSAPP_PROXY never reach the logs. A parse without a host counts as
// unparseable: "user:pass@host:1080" reads as scheme "user" with the
// credentials left in Opaque, which Redacted would print unchanged.
func redactProxyURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "<unparseable-proxy-url>"
	}
	if u.User != nil {
		// Redacted keeps the username, which can be an access token, and
		// url.User("***") would print escaped, so write the mask as text.
		u.User = nil
		return strings.Replace(u.String(), "//", "//***@", 1)
	}
	return u.Redacted()
}
