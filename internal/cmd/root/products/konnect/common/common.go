package common

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/konnect/auth"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/konnect/httpclient"
	"github.com/kong/kongctl/internal/meta"
	kprofile "github.com/kong/kongctl/internal/profile"
	viperutil "github.com/kong/kongctl/internal/util/viper"
	"github.com/spf13/cobra"
)

const (
	GlobalBaseURL   = "https://global.api.konghq.com"
	BaseURLDefault  = "https://us.api.konghq.com"
	BaseURLFlagName = "base-url"

	AuthBaseURLDefault  = GlobalBaseURL
	AuthBaseURLFlagName = "base-auth-url"

	AuthPathDefault  = "/v3/internal/oauth/device/authorize"
	AuthPathFlagName = "auth-path"

	TokenPathDefault  = "/v3/internal/oauth/device/token" // #nosec G101
	TokenPathFlagName = "token-path"                      // #nosec G101

	RefreshPathDefault  = "/kauth/api/v1/refresh"
	RefreshPathFlagName = "refresh-path"

	MachineClientIDDefault  = "344f59db-f401-4ce7-9407-00a0823fbacf"
	MachineClientIDFlagName = "machine-client-id"

	PATFlagName = "pat"

	RegionFlagName = "region"

	EnvironmentCom        = "com"
	EnvironmentProduction = "production"
	EnvironmentTech       = "tech"

	TechGlobalBaseURL       = "https://global.api.konghq.tech"
	TechBaseURLDefault      = "https://us.api.konghq.tech"
	TechMachineClientID     = "35b065db-8eaf-4584-9cb6-05b1daea0750"
	KonnectEnvFlagName      = "konnect-env"
	konnectProductionDomain = "konghq.com"
	konnectTechDomain       = "konghq.tech"

	RequestPageSizeFlagName = "page-size"
	DefaultRequestPageSize  = 10
)

var (
	PATConfigPath          = "konnect." + PATFlagName
	AuthTokenConfigPath    = "konnect.auth-token"    // #nosec G101
	RefreshTokenConfigPath = "konnect.refresh-token" // #nosec G101

	BaseURLConfigPath             = "konnect." + BaseURLFlagName
	AuthBaseURLConfigPath         = "konnect." + AuthBaseURLFlagName
	AuthPathConfigPath            = "konnect." + AuthPathFlagName
	AuthMachineClientIDConfigPath = "konnect." + MachineClientIDFlagName
	TokenURLPathConfigPath        = "konnect." + TokenPathFlagName
	RefreshPathConfigPath         = "konnect." + RefreshPathFlagName

	MachineClientIDConfigPath = "konnect." + MachineClientIDFlagName
	RequestPageSizeConfigPath = "konnect." + RequestPageSizeFlagName

	RegionConfigPath      = "konnect." + RegionFlagName
	EnvironmentConfigPath = "konnect.environment"

	HTTPRetryMaxAttemptsConfigPath        = "konnect." + cmdcommon.HTTPRetryMaxAttemptsConfigPath
	HTTPRetryInitialIntervalConfigPath    = "konnect." + cmdcommon.HTTPRetryInitialIntervalConfigPath
	HTTPRetryMaxIntervalConfigPath        = "konnect." + cmdcommon.HTTPRetryMaxIntervalConfigPath
	HTTPRetryBackoffFactorConfigPath      = "konnect." + cmdcommon.HTTPRetryBackoffFactorConfigPath
	HTTPRetryOnConnectionErrorsConfigPath = "konnect." + cmdcommon.HTTPRetryOnConnectionErrorsConfigPath
)

var regionPattern = regexp.MustCompile(`^[a-z0-9-]+$`)

type EnvironmentDefaults struct {
	Name            string
	BaseURL         string
	AuthBaseURL     string
	MachineClientID string
}

func ProductionEnvironmentDefaults() EnvironmentDefaults {
	return EnvironmentDefaults{
		Name:            EnvironmentProduction,
		BaseURL:         BaseURLDefault,
		AuthBaseURL:     AuthBaseURLDefault,
		MachineClientID: MachineClientIDDefault,
	}
}

func TechEnvironmentDefaults() EnvironmentDefaults {
	return EnvironmentDefaults{
		Name:            EnvironmentTech,
		BaseURL:         TechBaseURLDefault,
		AuthBaseURL:     TechGlobalBaseURL,
		MachineClientID: TechMachineClientID,
	}
}

func EnvironmentDefaultsFor(name string) (EnvironmentDefaults, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", EnvironmentCom, EnvironmentProduction, "prod":
		return ProductionEnvironmentDefaults(), nil
	case EnvironmentTech:
		return TechEnvironmentDefaults(), nil
	default:
		return EnvironmentDefaults{}, fmt.Errorf("unsupported konnect environment %q", name)
	}
}

func ApplyEnvironmentDefaults(command *cobra.Command, cfg config.Hook) error {
	if command == nil || cfg == nil {
		return nil
	}

	defaults, selected, err := SelectedEnvironmentDefaults(command, cfg)
	if err != nil || !selected {
		return err
	}

	if shouldApplyEnvironmentEndpointDefault(command, cfg, BaseURLFlagName, BaseURLConfigPath) {
		baseURL := defaults.BaseURL
		if region, ok := cmdcommon.CommandTreeChangedFlagString(command, RegionFlagName); ok {
			resolved, err := BuildBaseURLFromRegionForEnvironment(region, defaults.Name)
			if err != nil {
				return err
			}
			baseURL = resolved
		} else if region := strings.TrimSpace(cfg.GetString(RegionConfigPath)); region != "" {
			resolved, err := BuildBaseURLFromRegionForEnvironment(region, defaults.Name)
			if err != nil {
				return err
			}
			baseURL = resolved
		}
		cfg.SetString(BaseURLConfigPath, baseURL)
		if err := cmdcommon.SetCommandTreeFlagValue(command, BaseURLFlagName, baseURL); err != nil {
			return err
		}
	}
	if shouldApplyEnvironmentEndpointDefault(command, cfg, AuthBaseURLFlagName, AuthBaseURLConfigPath) {
		cfg.SetString(AuthBaseURLConfigPath, defaults.AuthBaseURL)
		if err := cmdcommon.SetCommandTreeFlagValue(command, AuthBaseURLFlagName, defaults.AuthBaseURL); err != nil {
			return err
		}
	}
	if shouldApplyEnvironmentEndpointDefault(command, cfg, MachineClientIDFlagName, MachineClientIDConfigPath) {
		cfg.SetString(MachineClientIDConfigPath, defaults.MachineClientID)
		if err := cmdcommon.SetCommandTreeFlagValue(command, MachineClientIDFlagName, defaults.MachineClientID); err != nil {
			return err
		}
	}
	return nil
}

func SelectedEnvironmentDefaults(command *cobra.Command, cfg config.Hook) (EnvironmentDefaults, bool, error) {
	if value, ok := cmdcommon.CommandTreeChangedFlagString(command, KonnectEnvFlagName); ok {
		defaults, err := environmentDefaultsForSelector(value)
		return defaults, true, err
	}

	if cfg == nil {
		return EnvironmentDefaults{}, false, nil
	}

	value := cfg.GetString(EnvironmentConfigPath)
	if strings.TrimSpace(value) == "" {
		return EnvironmentDefaults{}, false, nil
	}
	defaults, err := environmentDefaultsForSelector(value)
	return defaults, true, err
}

func environmentDefaultsForSelector(value string) (EnvironmentDefaults, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case EnvironmentCom, EnvironmentProduction, "prod", EnvironmentTech:
		return EnvironmentDefaultsFor(normalized)
	default:
		return EnvironmentDefaults{},
			fmt.Errorf("unsupported konnect environment %q (allowed: %s, %s; aliases: %s, prod)",
				value, EnvironmentProduction, EnvironmentTech, EnvironmentCom)
	}
}

func shouldApplyEnvironmentEndpointDefault(command *cobra.Command, cfg config.Hook, flagName, configPath string) bool {
	if cmdcommon.CommandTreeFlagChanged(command, flagName) {
		return false
	}
	value := strings.TrimSpace(cfg.GetString(configPath))
	if value == "" {
		return true
	}
	return !isExplicitEndpointConfig(cfg, configPath, value)
}

func isExplicitEndpointConfig(cfg config.Hook, configPath, value string) bool {
	if cfg.InConfig(configPath) || profileConfigEnvSet(cfg.GetProfile(), configPath) {
		return true
	}
	return !isKnownEnvironmentDefault(configPath, value)
}

func profileConfigEnvSet(profile, configPath string) bool {
	profile = strings.TrimSpace(profile)
	configPath = strings.TrimSpace(configPath)
	if profile == "" || configPath == "" {
		return false
	}
	name := "KONGCTL_" + strings.ToUpper(strings.ReplaceAll(profile, "-", "_")) + "_" +
		strings.ToUpper(strings.NewReplacer(".", "_", "-", "_").Replace(configPath))
	value, ok := os.LookupEnv(name)
	return ok && strings.TrimSpace(value) != ""
}

func isKnownEnvironmentDefault(configPath, value string) bool {
	switch configPath {
	case BaseURLConfigPath:
		return value == BaseURLDefault || value == TechBaseURLDefault ||
			value == GlobalBaseURL || value == TechGlobalBaseURL
	case AuthBaseURLConfigPath:
		return value == AuthBaseURLDefault || value == TechGlobalBaseURL
	case MachineClientIDConfigPath:
		return value == MachineClientIDDefault || value == TechMachineClientID
	default:
		return false
	}
}

func InferEnvironmentDefaultsFromURL(rawURL string) (EnvironmentDefaults, bool) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return EnvironmentDefaults{}, false
	}
	host := strings.ToLower(parsed.Hostname())
	switch {
	case host == konnectTechDomain || strings.HasSuffix(host, "."+konnectTechDomain):
		return TechEnvironmentDefaults(), true
	case host == konnectProductionDomain || strings.HasSuffix(host, "."+konnectProductionDomain):
		return ProductionEnvironmentDefaults(), true
	default:
		return EnvironmentDefaults{}, false
	}
}

// BuildBaseURLFromRegion converts a region identifier into the corresponding Konnect API host.
func BuildBaseURLFromRegion(region string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(region))
	if trimmed == "" {
		return "", fmt.Errorf("konnect region cannot be empty")
	}

	if trimmed == "global" {
		return GlobalBaseURL, nil
	}

	if !regionPattern.MatchString(trimmed) {
		return "", fmt.Errorf("invalid konnect region %q (expected lowercase letters, numbers, or hyphens)", region)
	}

	return fmt.Sprintf("https://%s.api.konghq.com", trimmed), nil
}

func BuildBaseURLFromRegionForEnvironment(region string, environment string) (string, error) {
	defaults, err := EnvironmentDefaultsFor(environment)
	if err != nil {
		return "", err
	}

	baseURL, err := BuildBaseURLFromRegion(region)
	if err != nil {
		return "", err
	}
	if defaults.Name == EnvironmentTech {
		return strings.Replace(baseURL, konnectProductionDomain, konnectTechDomain, 1), nil
	}
	return baseURL, nil
}

// ResolveBaseURL determines the effective Konnect base URL, honoring the precedence rules:
// 1) Explicit base-url flag/config
// 2) Region flag/config (converted to a URL)
// 3) Default US region
func ResolveBaseURL(cfg config.Hook) (string, error) {
	baseURL := strings.TrimSpace(cfg.GetString(BaseURLConfigPath))
	if baseURL != "" {
		return baseURL, nil
	}

	region := strings.TrimSpace(cfg.GetString(RegionConfigPath))
	var resolved string
	if region != "" {
		r, err := BuildBaseURLFromRegion(region)
		if err != nil {
			return "", err
		}
		resolved = r
	} else {
		resolved = BaseURLDefault
	}

	cfg.SetString(BaseURLConfigPath, resolved)
	return resolved, nil
}

func ResolveHTTPTimeout(cfg config.Hook) (time.Duration, error) {
	timeout, set, err := resolveOptionalDuration(cfg, cmdcommon.HTTPTimeoutConfigPath)
	if err != nil {
		return 0, err
	}
	if !set {
		return httpclient.DefaultHTTPClientTimeout, nil
	}
	return timeout, nil
}

func ResolveHTTPTransportOptions(cfg config.Hook) (httpclient.TransportOptions, error) {
	tcpUserTimeout, _, err := resolveOptionalDuration(cfg, cmdcommon.HTTPTCPUserTimeoutConfigPath)
	if err != nil {
		return httpclient.TransportOptions{}, err
	}

	disableKeepAlives, err := resolveOptionalBool(cfg, cmdcommon.HTTPDisableKeepAlivesConfigPath)
	if err != nil {
		return httpclient.TransportOptions{}, err
	}

	recycleOnError, err := resolveOptionalBool(cfg, cmdcommon.HTTPRecycleConnectionsOnErrorConfigPath)
	if err != nil {
		return httpclient.TransportOptions{}, err
	}

	return httpclient.TransportOptions{
		TCPUserTimeout:            tcpUserTimeout,
		DisableKeepAlives:         disableKeepAlives,
		RecycleConnectionsOnError: recycleOnError,
	}, nil
}

// ResolveRetryConfig builds an httpclient.RetryConfig from configuration,
// applying defaults when values are unset.
func ResolveRetryConfig(cfg config.Hook) (httpclient.RetryConfig, error) {
	maxAttempts, err := resolveOptionalInt(cfg, HTTPRetryMaxAttemptsConfigPath)
	if err != nil {
		return httpclient.RetryConfig{}, err
	}
	if maxAttempts < 0 {
		return httpclient.RetryConfig{}, fmt.Errorf("invalid %s value %d: must be >= 0",
			HTTPRetryMaxAttemptsConfigPath, maxAttempts)
	}
	if maxAttempts > httpclient.MaxRetryMaxAttempts {
		return httpclient.RetryConfig{}, fmt.Errorf(
			"invalid %s value %d: must be <= %d",
			HTTPRetryMaxAttemptsConfigPath, maxAttempts, httpclient.MaxRetryMaxAttempts,
		)
	}
	if maxAttempts == 0 {
		maxAttempts = httpclient.DefaultRetryMaxAttempts
	}

	strategy := httpclient.RetryStrategyBackoff
	if maxAttempts == 1 {
		strategy = httpclient.RetryStrategyNone
	}

	initialIntervalMS, err := resolveOptionalInt(cfg, HTTPRetryInitialIntervalConfigPath)
	if err != nil {
		return httpclient.RetryConfig{}, err
	}
	if initialIntervalMS == 0 {
		initialIntervalMS = httpclient.DefaultRetryInitialIntervalMS
	}
	if initialIntervalMS < httpclient.MinRetryInitialIntervalMS {
		return httpclient.RetryConfig{}, fmt.Errorf(
			"invalid %s value %d: must be >= %d ms",
			HTTPRetryInitialIntervalConfigPath, initialIntervalMS, httpclient.MinRetryInitialIntervalMS,
		)
	}
	if initialIntervalMS > httpclient.MaxRetryInitialIntervalMS {
		return httpclient.RetryConfig{}, fmt.Errorf(
			"invalid %s value %d: must be <= %d ms",
			HTTPRetryInitialIntervalConfigPath, initialIntervalMS, httpclient.MaxRetryInitialIntervalMS,
		)
	}

	maxIntervalMS, err := resolveOptionalInt(cfg, HTTPRetryMaxIntervalConfigPath)
	if err != nil {
		return httpclient.RetryConfig{}, err
	}
	if maxIntervalMS == 0 {
		maxIntervalMS = httpclient.DefaultRetryMaxIntervalMS
	}
	if maxIntervalMS < httpclient.MinRetryMaxIntervalMS {
		return httpclient.RetryConfig{}, fmt.Errorf(
			"invalid %s value %d: must be >= %d ms",
			HTTPRetryMaxIntervalConfigPath, maxIntervalMS, httpclient.MinRetryMaxIntervalMS,
		)
	}
	if maxIntervalMS > httpclient.MaxRetryMaxIntervalMS {
		return httpclient.RetryConfig{}, fmt.Errorf(
			"invalid %s value %d: must be <= %d ms",
			HTTPRetryMaxIntervalConfigPath, maxIntervalMS, httpclient.MaxRetryMaxIntervalMS,
		)
	}
	if initialIntervalMS > maxIntervalMS {
		return httpclient.RetryConfig{}, fmt.Errorf(
			"invalid configuration: %s (%d ms) must be <= %s (%d ms)",
			HTTPRetryInitialIntervalConfigPath, initialIntervalMS,
			HTTPRetryMaxIntervalConfigPath, maxIntervalMS,
		)
	}

	factor, err := resolveOptionalFloat64(cfg, HTTPRetryBackoffFactorConfigPath)
	if err != nil {
		return httpclient.RetryConfig{}, err
	}
	if factor == 0 {
		factor = httpclient.DefaultRetryBackoffFactor
	}
	if math.IsNaN(factor) || math.IsInf(factor, 0) {
		return httpclient.RetryConfig{}, fmt.Errorf("invalid %s value %g: must be a finite number",
			HTTPRetryBackoffFactorConfigPath, factor)
	}
	if factor < httpclient.MinRetryBackoffFactor {
		return httpclient.RetryConfig{}, fmt.Errorf("invalid %s value %g: must be >= %g",
			HTTPRetryBackoffFactorConfigPath, factor, httpclient.MinRetryBackoffFactor)
	}
	if factor > httpclient.MaxRetryBackoffFactor {
		return httpclient.RetryConfig{}, fmt.Errorf("invalid %s value %g: must be <= %g",
			HTTPRetryBackoffFactorConfigPath, factor, httpclient.MaxRetryBackoffFactor)
	}

	retryConnErrors, err := resolveOptionalBool(cfg, HTTPRetryOnConnectionErrorsConfigPath)
	if err != nil {
		return httpclient.RetryConfig{}, err
	}

	retryConfig := httpclient.RetryConfig{
		Strategy:              strategy,
		MaxAttempts:           maxAttempts,
		InitialIntervalMS:     initialIntervalMS,
		MaxIntervalMS:         maxIntervalMS,
		BackoffFactor:         factor,
		RetryConnectionErrors: retryConnErrors,
	}
	totalBackoffMS := httpclient.EstimatedRetryBackoffMS(retryConfig)
	if totalBackoffMS > httpclient.MaxRetryTotalBackoffMS {
		return httpclient.RetryConfig{}, fmt.Errorf(
			"invalid retry configuration: cumulative backoff budget %d ms must be <= %d ms",
			totalBackoffMS, httpclient.MaxRetryTotalBackoffMS,
		)
	}

	return retryConfig, nil
}

func GetAccessToken(cfg config.Hook, logger *slog.Logger) (string, error) {
	source, err := GetAccessTokenSource(cfg, logger)
	if err != nil {
		return "", err
	}

	return ResolveAccessToken(context.Background(), cfg, source)
}

func ResolveAccessToken(ctx context.Context, cfg config.Hook, source *auth.TokenSource) (string, error) {
	if source == nil {
		return "", accessTokenUnavailableError(cfg)
	}

	token, err := source.Token(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return "", err
		}
		return "", accessTokenUnavailableError(cfg)
	}
	return token, nil
}

func GetAccessTokenSource(cfg config.Hook, logger *slog.Logger) (*auth.TokenSource, error) {
	pat := cfg.GetString(PATConfigPath)
	if pat != "" {
		return auth.NewTokenSource(cfg, auth.TokenSourceOptions{
			PAT:    pat,
			Logger: logger,
		}), nil
	}

	baseURL, err := ResolveBaseURL(cfg)
	if err != nil {
		return nil, err
	}

	refreshPath := cfg.GetString(RefreshPathConfigPath)
	if refreshPath == "" {
		refreshPath = RefreshPathDefault
	}
	refreshURL := baseURL + refreshPath

	timeout, err := ResolveHTTPTimeout(cfg)
	if err != nil {
		return nil, err
	}

	transportOptions, err := ResolveHTTPTransportOptions(cfg)
	if err != nil {
		return nil, err
	}

	return auth.NewTokenSource(cfg, auth.TokenSourceOptions{
		PAT:              pat,
		RefreshURL:       refreshURL,
		Timeout:          timeout,
		TransportOptions: transportOptions,
		Logger:           logger,
	}), nil
}

func accessTokenUnavailableError(cfg config.Hook) error {
	profile := cfg.GetProfile()

	if !profileIsConfigured(cfg) {
		return unknownProfileError(cfg, profile)
	}

	envVar := fmt.Sprintf("KONGCTL_%s_KONNECT_PAT", strings.ToUpper(profile))

	return fmt.Errorf(
		"authentication token not available. Use one of the following to authorize %s:\n"+
			"  - '%s login' to authenticate via the web\n"+
			"  - provide a token via the --%s flag\n"+
			"  - set the %s environment variable\n"+
			"  - configure a token value in the '%s.%s' path of your configuration file",
		meta.CLIName,
		meta.CLIName,
		PATFlagName,
		envVar,
		profile,
		PATConfigPath,
	)
}

// profileIsConfigured reports whether the CLI has any prior knowledge of the
// requested profile, via the config file, environment variables, or a stored
// login credential. An unknown profile is almost always a mistyped --profile
// value, which otherwise surfaces later as a confusing auth failure.
func profileIsConfigured(cfg config.Hook) bool {
	name := cfg.GetProfile()
	if name == kprofile.DefaultProfile {
		return true
	}
	if slices.Contains(configuredProfileNames(cfg), name) {
		return true
	}
	if profileHasEnvVars(name) {
		return true
	}
	return auth.HasStoredCredential(cfg)
}

// unknownProfileError explains that the requested profile is unknown and lists
// the profiles found in the config file so a mistyped name is easy to spot.
func unknownProfileError(cfg config.Hook, profile string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "profile %q is not configured.\n\n", profile)

	available := configuredProfileNames(cfg)
	if len(available) > 0 {
		b.WriteString("Available profiles:\n")
		for _, name := range available {
			fmt.Fprintf(&b, "    %s\n", name)
		}
		fmt.Fprintf(&b, "\nUse one of these profiles or run '%s login --profile %s' "+
			"to authenticate with this profile.", meta.CLIName, profile)
	} else {
		b.WriteString("No profiles are configured yet.\n\n")
		fmt.Fprintf(&b, "Run '%s login --profile %s' to authenticate with this profile.",
			meta.CLIName, profile)
	}

	return errors.New(b.String())
}

// configuredProfileNames returns the sorted profiles defined in the config file.
func configuredProfileNames(cfg config.Hook) []string {
	pc, ok := cfg.(*config.ProfiledConfig)
	if !ok || pc.Viper == nil {
		return nil
	}
	names := kprofile.NewManager(pc.Viper).GetProfiles()
	slices.Sort(names)
	return names
}

func profileHasEnvVars(name string) bool {
	prefix := viperutil.ProfileEnvPrefix(name) + "_"
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, prefix) {
			return true
		}
	}
	return false
}

func isDeclarativeRetryVerb(verb verbs.VerbValue) bool {
	return verb == verbs.Plan || verb == verbs.Sync || verb == verbs.Diff ||
		verb == verbs.Export || verb == verbs.Apply || verb == verbs.Delete
}

func noRetryConfig() httpclient.RetryConfig {
	return httpclient.RetryConfig{
		Strategy:              httpclient.RetryStrategyNone,
		MaxAttempts:           1,
		InitialIntervalMS:     httpclient.DefaultRetryInitialIntervalMS,
		MaxIntervalMS:         httpclient.DefaultRetryMaxIntervalMS,
		BackoffFactor:         httpclient.DefaultRetryBackoffFactor,
		RetryConnectionErrors: false,
	}
}

func resolveRetryConfigForVerb(cfg config.Hook, verb verbs.VerbValue) (httpclient.RetryConfig, error) {
	if !isDeclarativeRetryVerb(verb) {
		return noRetryConfig(), nil
	}

	return ResolveRetryConfig(cfg)
}

func konnectSDKFactory(
	cfg config.Hook,
	logger *slog.Logger,
	retryConfig httpclient.RetryConfig,
) (helpers.SDKAPI, error) {
	tokenSource, e := GetAccessTokenSource(cfg, logger)
	if e != nil {
		return nil, e
	}
	token, e := ResolveAccessToken(context.Background(), cfg, tokenSource)
	if e != nil {
		return nil, e
	}

	baseURL, err := ResolveBaseURL(cfg)
	if err != nil {
		return nil, err
	}

	timeout, err := ResolveHTTPTimeout(cfg)
	if err != nil {
		return nil, err
	}

	transportOptions, err := ResolveHTTPTransportOptions(cfg)
	if err != nil {
		return nil, err
	}

	sdk, httpClient, err := auth.GetAuthenticatedClient(
		baseURL, tokenSource, timeout, transportOptions, &retryConfig, logger,
	)
	if err != nil {
		return nil, err
	}

	return &helpers.KonnectSDK{
		SDK:         sdk,
		BaseURL:     baseURL,
		Token:       token,
		TokenSource: tokenSource,
		HTTPClient:  httpClient,
	}, nil
}

// This is the real implementation of the SDKAPIFactory,
// which creates a real Konnect SDK instance
func KonnectSDKFactory(cfg config.Hook, logger *slog.Logger) (helpers.SDKAPI, error) {
	return konnectSDKFactory(cfg, logger, noRetryConfig())
}

func KonnectSDKFactoryForVerb(verb verbs.VerbValue, cfg config.Hook, logger *slog.Logger) (helpers.SDKAPI, error) {
	retryConfig, err := resolveRetryConfigForVerb(cfg, verb)
	if err != nil {
		return nil, err
	}

	return konnectSDKFactory(cfg, logger, retryConfig)
}

func GetSDKFactoryForVerb(verb verbs.VerbValue) helpers.SDKAPIFactory {
	if helpers.DefaultSDKFactory != nil {
		return helpers.DefaultSDKFactory
	}

	return func(cfg config.Hook, logger *slog.Logger) (helpers.SDKAPI, error) {
		return KonnectSDKFactoryForVerb(verb, cfg, logger)
	}
}

// GetSDKFactory returns the SDK factory to use, checking for test overrides.
// The returned factory uses no retry config. Call GetSDKFactoryForVerb to
// enable retries for declarative verbs.
func GetSDKFactory() helpers.SDKAPIFactory {
	if helpers.DefaultSDKFactory != nil {
		return helpers.DefaultSDKFactory
	}
	return KonnectSDKFactory
}

func resolveOptionalDuration(cfg config.Hook, configPath string) (time.Duration, bool, error) {
	raw := strings.TrimSpace(cfg.GetString(configPath))
	if raw == "" {
		return 0, false, nil
	}
	if timeoutDisabled(raw) {
		return 0, true, nil
	}

	value, err := time.ParseDuration(raw)
	if err != nil || value < 0 {
		return 0, true, fmt.Errorf("invalid %s value %q", configPath, raw)
	}

	return value, true, nil
}

func timeoutDisabled(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "0", "default", "defaults", "disable", "disabled", "none", "off", "platform", "system":
		return true
	default:
		return false
	}
}

func resolveOptionalBool(cfg config.Hook, configPath string) (bool, error) {
	raw := strings.TrimSpace(cfg.GetString(configPath))
	if raw == "" {
		return false, nil
	}

	switch strings.ToLower(raw) {
	case "1", "true", "yes", "on", "y":
		return true, nil
	case "0", "false", "no", "off", "n":
		return false, nil
	default:
		return false, fmt.Errorf("invalid %s value %q", configPath, raw)
	}
}

func resolveOptionalInt(cfg config.Hook, configPath string) (int, error) {
	raw := strings.TrimSpace(cfg.GetString(configPath))
	if raw == "" {
		return 0, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s value %q: must be an integer", configPath, raw)
	}
	return v, nil
}

func resolveOptionalFloat64(cfg config.Hook, configPath string) (float64, error) {
	raw := strings.TrimSpace(cfg.GetString(configPath))
	if raw == "" {
		return 0, nil
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s value %q: must be a number", configPath, raw)
	}
	return v, nil
}
