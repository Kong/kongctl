//go:build e2e

package harness

import (
	"fmt"
	"os"
	"strings"

	konnectcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
)

const (
	KonnectEnvironmentEnvName     = "KONGCTL_E2E_KONNECT_ENV"
	KonnectBaseURLEnvName         = "KONGCTL_E2E_KONNECT_BASE_URL"
	KonnectBaseAuthURLEnvName     = "KONGCTL_E2E_KONNECT_BASE_AUTH_URL"
	KonnectMachineClientIDEnvName = "KONGCTL_E2E_KONNECT_MACHINE_CLIENT_ID"
)

type KonnectTarget struct {
	Name            string
	BaseURL         string
	BaseAuthURL     string
	MachineClientID string
}

func ResolveKonnectTargetFromEnv() (KonnectTarget, error) {
	defaults, err := konnectcommon.EnvironmentDefaultsFor(os.Getenv(KonnectEnvironmentEnvName))
	if err != nil {
		return KonnectTarget{}, err
	}

	target := KonnectTarget{
		Name:            defaults.Name,
		BaseURL:         defaults.BaseURL,
		BaseAuthURL:     defaults.AuthBaseURL,
		MachineClientID: defaults.MachineClientID,
	}

	baseURL := strings.TrimSpace(os.Getenv(KonnectBaseURLEnvName))
	baseAuthURL := strings.TrimSpace(os.Getenv(KonnectBaseAuthURLEnvName))
	machineClientID := strings.TrimSpace(os.Getenv(KonnectMachineClientIDEnvName))

	if baseURL != "" {
		target.BaseURL = baseURL
		if inferred, ok := konnectcommon.InferEnvironmentDefaultsFromURL(baseURL); ok {
			target.Name = inferred.Name
			if baseAuthURL == "" {
				target.BaseAuthURL = inferred.AuthBaseURL
			}
			if machineClientID == "" {
				target.MachineClientID = inferred.MachineClientID
			}
		}
	}
	if baseAuthURL != "" {
		target.BaseAuthURL = baseAuthURL
		if inferred, ok := konnectcommon.InferEnvironmentDefaultsFromURL(baseAuthURL); ok && machineClientID == "" {
			target.Name = inferred.Name
			target.MachineClientID = inferred.MachineClientID
		}
	}
	if machineClientID != "" {
		target.MachineClientID = machineClientID
	}

	return target, nil
}

func KonnectBaseURL() (string, error) {
	target, err := ResolveKonnectTargetFromEnv()
	if err != nil {
		return "", err
	}
	return target.BaseURL, nil
}

func KonnectBaseAuthURL() (string, error) {
	target, err := ResolveKonnectTargetFromEnv()
	if err != nil {
		return "", err
	}
	return target.BaseAuthURL, nil
}

func KonnectBaseURLFromRegion(region string) (string, error) {
	r := strings.TrimSpace(region)
	if r == "" {
		return "", fmt.Errorf("konnect region cannot be empty")
	}
	if strings.HasPrefix(r, "http://") || strings.HasPrefix(r, "https://") {
		return r, nil
	}

	target, err := ResolveKonnectTargetFromEnv()
	if err != nil {
		return "", err
	}
	return konnectcommon.BuildBaseURLFromRegionForEnvironment(r, target.Name)
}
