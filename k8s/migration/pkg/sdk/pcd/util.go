package du

import (
	"errors"
	"os"
	"strings"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
)

func ParseInfoFromEnv() (Info, error) {
	du := Info{
		URL:      strings.TrimSpace(os.Getenv("DU_URL")),
		Insecure: os.Getenv("DU_INSECURE") == "true",
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

func ParseInfoFromOpenstackCreds(openstackCreds vjailbreakv1alpha1.OpenStackCredsInfo) (Info, error) {
	return Info{
		URL:      strings.Join(strings.Split(openstackCreds.AuthURL, "/")[:3], "/"),
		Insecure: openstackCreds.Insecure,
	}, nil
}
