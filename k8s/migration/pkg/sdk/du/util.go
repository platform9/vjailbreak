package du

import (
	"errors"
	"os"
	"strings"
)

func ParseInfoFromEnv() (Info, error) {
	du := Info{
		URL:               strings.TrimSpace(os.Getenv("DU_URL")),
		ApiserverEndpoint: strings.TrimSpace(os.Getenv("DU_APISERVER_ENDPOINT")),
		ForwarderEndpoint: strings.TrimSpace(os.Getenv("DU_FORWARDER_ENDPOINT")),
	}

	// TODO: Default 'ApiserverEndpoint' and 'ForwarderEndpoint'
	if len(du.URL) == 0 {
		return du, errors.New("environment variable DU_URL is required")
	}

	if !strings.HasPrefix(du.URL, "http") {
		du.URL = "https://" + du.URL
	}
	return du, nil
}
