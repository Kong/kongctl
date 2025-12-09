package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	sdkkonnectgo "github.com/Kong/sdk-konnect-go"
	kkcomponents "github.com/Kong/sdk-konnect-go/models/components"
	kkoperations "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/google/uuid"
	"github.com/kong/kongctl/test/e2e/harness/portalclient"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func main() {
	var (
		baseURL = flag.String(
			"base-url",
			"",
			"Base URL for the developer portal (for example, https://portal.example.com)",
		)
		developerEmail      = flag.String("developer-email", "", "Developer email used for registration and authentication")
		developerName       = flag.String("developer-name", "Test Developer", "Developer full name")
		developerPass       = flag.String("developer-password", "", "Developer password used for authentication")
		applicationName     = flag.String("application-name", "e2e-portal-application", "Application name to create")
		portalID            = flag.String("portal-id", "", "Konnect portal ID (used for administrative approval)")
		konnectBaseURL      = flag.String("konnect-base-url", "", "Konnect API base URL (optional)")
		registrationAPIName = flag.String(
			"registration-api-name",
			"",
			"API name to register the application to (optional)",
		)
	)
	flag.Parse()

	envEmail := strings.TrimSpace(os.Getenv("KONGCTL_E2E_GMAIL_ADDRESS"))

	if err := validateFlags(*baseURL, *developerEmail, *developerPass, *applicationName, envEmail); err != nil {
		log.Fatalf("invalid input: %v", err)
	}

	gmailClient, err := newGmailClientFromEnv()
	if err != nil {
		log.Fatalf("gmail configuration invalid: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	log.Printf("initializing portal client (base=%s)", strings.TrimSpace(*baseURL))
	api, err := portalclient.NewPortalAPI(*baseURL)
	if err != nil {
		log.Fatalf("failed to initialize portal client: %v", err)
	}
	log.Printf("portal client ready")

	emailToUse := stringsTrim(*developerEmail)
	if emailToUse == "" {
		emailToUse = randomGmailAddress(envEmail)
		log.Printf("generated developer email %s", emailToUse)
	} else {
		log.Printf("using provided developer email %s", emailToUse)
	}

	log.Printf("registering developer %s", emailToUse)
	if err := retry(ctx, 6, 5*time.Second, func(ctx context.Context) error {
		return registerDeveloper(ctx, api, emailToUse, *developerName)
	}); err != nil {
		log.Fatalf("register developer failed: %v", err)
	}
	log.Printf("developer registration succeeded")

	log.Printf("waiting for verification email for %s", emailToUse)
	if err := completeDeveloperVerification(ctx, api, gmailClient, emailToUse, *developerPass); err != nil {
		log.Fatalf("developer verification failed: %v", err)
	}
	log.Printf("developer email verification and password setup succeeded")

	if strings.TrimSpace(*portalID) != "" {
		if err := approveDeveloper(ctx, *portalID, emailToUse, *konnectBaseURL); err != nil {
			log.Fatalf("approve developer failed: %v", err)
		}
		log.Printf("developer approved via Konnect API")
	}

	log.Printf("authenticating developer %s", emailToUse)
	if err := retry(ctx, 6, 5*time.Second, func(ctx context.Context) error {
		return authenticateDeveloper(ctx, api, emailToUse, *developerPass)
	}); err != nil {
		log.Fatalf("developer authentication failed: %v", err)
	}
	log.Printf("developer authentication succeeded")

	authStrategyID, err := resolveAuthStrategyID(ctx, api)
	if err != nil {
		log.Fatalf("resolve auth strategy failed: %v", err)
	}
	if authStrategyID != nil {
		log.Printf("using auth strategy %s", authStrategyID.String())
	}

	log.Printf("creating application %s", *applicationName)
	appID, err := retryCreateApplication(ctx, api, *applicationName, authStrategyID)
	if err != nil {
		log.Fatalf("create application failed: %v", err)
	}
	log.Printf("application created (id=%s)", appID)

	var registrationID string
	if name := stringsTrim(*registrationAPIName); name != "" {
		log.Printf("resolving API %q for registration", name)
		apiID, err := resolveAPIID(ctx, api, name)
		if err != nil {
			log.Fatalf("resolve API for registration failed: %v", err)
		}
		log.Printf("creating application registration for API %s", apiID)
		registrationID, err = createApplicationRegistration(ctx, api, appID, apiID)
		if err != nil {
			log.Fatalf("create application registration failed: %v", err)
		}
		log.Printf("application registration created (id=%s)", registrationID)
	}

	result := map[string]string{
		"developer_email":  emailToUse,
		"application_name": *applicationName,
		"application_id":   appID,
	}
	if registrationID != "" {
		result["registration_id"] = registrationID
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		log.Fatalf("failed to write result: %v", err)
	}
}

func resolveAPIID(ctx context.Context, api *portalclient.PortalAPI, apiName string) (string, error) {
	res, err := api.Raw().ListApisWithResponse(ctx, nil)
	if err != nil {
		return "", err
	}
	if res.JSON200 == nil {
		return "", fmt.Errorf("list apis returned no data (status %d)", res.StatusCode())
	}

	matches := make([]portalclient.Api, 0)
	for _, apiItem := range res.JSON200.Data {
		if strings.EqualFold(apiItem.Name, apiName) {
			matches = append(matches, apiItem)
		}
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("api %q not found", apiName)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("multiple apis found with name %q", apiName)
	}

	id := matches[0].Id
	if id == nil || stringsTrim(id.String()) == "" {
		return "", fmt.Errorf("api %q has no id", apiName)
	}

	return stringsTrim(id.String()), nil
}

func createApplicationRegistration(
	ctx context.Context, api *portalclient.PortalAPI, applicationID, apiID string,
) (string, error) {
	appUUID, err := uuid.Parse(applicationID)
	if err != nil {
		return "", fmt.Errorf("parse application id: %w", err)
	}
	apiUUID, err := uuid.Parse(apiID)
	if err != nil {
		return "", fmt.Errorf("parse api id: %w", err)
	}

	body := portalclient.CreateApplicationRegistrationJSONRequestBody{
		ApiId: apiUUID,
	}

	res, err := api.Raw().CreateApplicationRegistrationWithResponse(
		ctx,
		appUUID,
		body,
	)
	if err != nil {
		return "", err
	}

	if res.JSON201 == nil {
		return "", fmt.Errorf("unexpected response creating registration: status=%d body=%s",
			res.StatusCode(), strings.TrimSpace(string(res.Body)))
	}

	if res.JSON201.Id == nil {
		return "", fmt.Errorf("registration response missing id")
	}

	return stringsTrim(res.JSON201.Id.String()), nil
}

func randomGmailAddress(base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return fmt.Sprintf("kongctl-e2e+%d@example.com", time.Now().UnixNano())
	}
	local := base
	domain := ""
	if strings.Contains(base, "@") {
		parts := strings.SplitN(base, "@", 2)
		local = parts[0]
		domain = parts[1]
	}
	if idx := strings.Index(local, "+"); idx >= 0 {
		local = local[:idx]
	}
	if domain == "" {
		domain = "gmail.com"
	}
	return fmt.Sprintf("%s+portal-%d@%s", local, time.Now().UnixNano(), domain)
}

func validateFlags(baseURL, email, password, appName, gmailBase string) error {
	switch {
	case stringsTrim(baseURL) == "":
		return fmt.Errorf("base-url is required")
	case stringsTrim(email) == "" && stringsTrim(gmailBase) == "":
		return fmt.Errorf("either developer-email or KONGCTL_E2E_GMAIL_ADDRESS must be provided")
	case stringsTrim(password) == "":
		return fmt.Errorf("developer-password is required")
	case stringsTrim(appName) == "":
		return fmt.Errorf("application-name is required")
	default:
		return nil
	}
}

func stringsTrim(s string) string {
	return strings.TrimSpace(s)
}

func registerDeveloper(ctx context.Context, api *portalclient.PortalAPI, email, fullName string) error {
	payload := portalclient.RegisterPayload{
		Email:    openapi_types.Email(email),
		FullName: fullName,
	}

	res, err := api.Raw().RegisterWithResponse(ctx, payload)
	if err != nil {
		return err
	}

	switch res.StatusCode() {
	case 201, 202:
		return nil
	case 409:
		// developer already exists; treat as success so tests remain idempotent
		return nil
	default:
		return fmt.Errorf("unexpected register status %d: %s", res.StatusCode(), string(res.Body))
	}
}

func authenticateDeveloper(ctx context.Context, api *portalclient.PortalAPI, email, password string) error {
	emailValue := openapi_types.Email(email)
	payload := portalclient.AuthenticateRequest{
		Username: &emailValue,
		Password: &password,
	}

	res, err := api.Raw().AuthenticateWithResponse(ctx, payload)
	if err != nil {
		return err
	}
	if res.StatusCode() != 204 {
		bodySnippet := string(res.Body)
		if len(bodySnippet) > 256 {
			bodySnippet = bodySnippet[:256] + "…"
		}
		return fmt.Errorf("unexpected authenticate status %d body=%s", res.StatusCode(), strings.TrimSpace(bodySnippet))
	}
	return nil
}

func approveDeveloper(ctx context.Context, portalID, email, baseURLFlag string) error {
	pat := strings.TrimSpace(os.Getenv("KONGCTL_E2E_KONNECT_PAT"))
	if pat == "" {
		return fmt.Errorf("KONGCTL_E2E_KONNECT_PAT is required when --portal-id is provided")
	}

	baseURL := strings.TrimSpace(baseURLFlag)
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv("KONGCTL_E2E_KONNECT_BASE_URL"))
	}
	if baseURL == "" {
		baseURL = "https://us.api.konghq.com"
	}

	log.Printf("approving developer via Konnect API (portal=%s)", portalID)

	sdk := sdkkonnectgo.New(
		sdkkonnectgo.WithServerURL(baseURL),
		sdkkonnectgo.WithSecurity(kkcomponents.Security{
			PersonalAccessToken: &pat,
		}),
	)

	emailFilter := strings.TrimSpace(email)
	filter := &kkoperations.ListPortalDevelopersQueryParamFilter{
		Email: &kkcomponents.StringFieldFilter{
			Eq: &emailFilter,
		},
	}
	listReq := kkoperations.ListPortalDevelopersRequest{
		PortalID: portalID,
		Filter:   filter,
	}

	var (
		developerID string
		retryErr    error
	)
	retryErr = retry(ctx, 6, 5*time.Second, func(ctx context.Context) error {
		resp, err := sdk.PortalDevelopers.ListPortalDevelopers(ctx, listReq)
		if err != nil {
			return err
		}
		if resp.ListDevelopersResponse == nil {
			return fmt.Errorf("empty developer list response")
		}
		for _, dev := range resp.ListDevelopersResponse.Data {
			if strings.EqualFold(dev.GetEmail(), email) {
				log.Printf("found developer %s (id=%s status=%s)", email, dev.GetID(), dev.GetStatus())
				developerID = dev.GetID()
				break
			}
		}
		if developerID == "" {
			return fmt.Errorf("developer %s not yet visible", email)
		}
		return nil
	})
	if retryErr != nil {
		return fmt.Errorf("failed to find developer %s: %w", email, retryErr)
	}

	status := kkcomponents.DeveloperStatusApproved
	updateReq := kkoperations.UpdateDeveloperRequest{
		PortalID:    portalID,
		DeveloperID: developerID,
		UpdateDeveloperRequest: kkcomponents.UpdateDeveloperRequest{
			Status: status.ToPointer(),
		},
	}
	log.Printf("setting developer %s status to %s", developerID, status)
	if _, err := sdk.PortalDevelopers.UpdateDeveloper(ctx, updateReq); err != nil {
		return fmt.Errorf("update developer status failed: %w", err)
	}

	// ensure status change is visible before returning
	statusCheck := kkoperations.ListPortalDevelopersRequest{
		PortalID: portalID,
		Filter:   filter,
	}
	if err := retry(ctx, 6, 5*time.Second, func(ctx context.Context) error {
		resp, err := sdk.PortalDevelopers.ListPortalDevelopers(ctx, statusCheck)
		if err != nil {
			return err
		}
		if resp.ListDevelopersResponse == nil {
			return fmt.Errorf("empty developer list response")
		}
		for _, dev := range resp.ListDevelopersResponse.Data {
			if strings.EqualFold(dev.GetEmail(), email) && dev.GetStatus() == kkcomponents.DeveloperStatusApproved {
				log.Printf("developer %s status now %s", email, dev.GetStatus())
				return nil
			}
		}
		return fmt.Errorf("developer %s still pending", email)
	}); err != nil {
		return fmt.Errorf("developer approval not visible: %w", err)
	}
	// brief pause to allow portal caches to surface the new status
	time.Sleep(5 * time.Second)

	return nil
}

func retryCreateApplication(
	ctx context.Context,
	api *portalclient.PortalAPI,
	name string,
	authStrategyID *openapi_types.UUID,
) (string, error) {
	var appID string
	err := retry(ctx, 6, 5*time.Second, func(ctx context.Context) error {
		payload := portalclient.CreateApplicationPayload{Name: name}
		if authStrategyID != nil {
			payload.AuthStrategyId = authStrategyID
		}
		resp, err := api.Raw().CreateApplicationWithResponse(ctx, payload)
		if err != nil {
			return err
		}
		if resp.StatusCode() != 201 {
			body := ""
			if resp.Body != nil {
				body = strings.TrimSpace(string(resp.Body))
			}
			return fmt.Errorf("unexpected create application status %d body=%s", resp.StatusCode(), body)
		}
		if resp.JSON201 == nil || resp.JSON201.Id == nil {
			return fmt.Errorf("application response missing identifier")
		}
		appID = resp.JSON201.Id.String()
		return nil
	})
	return appID, err
}

func resolveAuthStrategyID(ctx context.Context, api *portalclient.PortalAPI) (*openapi_types.UUID, error) {
	if env := strings.TrimSpace(os.Getenv("KONGCTL_E2E_AUTH_STRATEGY_ID")); env != "" {
		id, err := parseUUID(env)
		if err != nil {
			return nil, fmt.Errorf("invalid KONGCTL_E2E_AUTH_STRATEGY_ID: %w", err)
		}
		return id, nil
	}

	resp, err := api.Raw().ListApplicationAuthStrategiesWithResponse(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("list auth strategies failed: %w", err)
	}
	if resp.JSON200 == nil || len(resp.JSON200.Data) == 0 {
		return nil, fmt.Errorf("no application auth strategies available")
	}
	for _, strategy := range resp.JSON200.Data {
		if id := extractAuthStrategyID(strategy); id != nil {
			return id, nil
		}
	}
	return nil, fmt.Errorf("no auth strategy provided IDs")
}

func extractAuthStrategyID(strategy portalclient.PortalAuthStrategy) *openapi_types.UUID {
	if key, err := strategy.AsPortalAuthStrategyKeyAuth(); err == nil && key.Id != nil {
		return key.Id
	}
	if cc, err := strategy.AsPortalAuthStrategyClientCredentials(); err == nil && cc.Id != nil {
		return cc.Id
	}
	return nil
}

//nolint:unparam // attempts can be variable
func retry(ctx context.Context, attempts int, delay time.Duration, fn func(context.Context) error) error {
	if attempts < 1 {
		attempts = 1
	}
	for attempt := 0; attempt < attempts; attempt++ {
		runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		err := fn(runCtx)
		cancel()
		if err == nil {
			return nil
		}
		if attempt+1 == attempts {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	return fmt.Errorf("retry attempts exhausted")
}
