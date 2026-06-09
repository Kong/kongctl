package auth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"

	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/konnect/httpclient"
)

const DefaultAccessTokenExpirySkew = time.Minute

var ErrTokenRefreshUnsupported = errors.New("token refresh is not supported")

// TokenSource provides the current Konnect bearer token for each request.
// It is safe for concurrent use by future parallel executor workers.
type TokenSource struct {
	cfg              config.Hook
	pat              string
	refreshURL       string
	timeout          time.Duration
	transportOptions httpclient.TransportOptions
	logger           *slog.Logger
	expirySkew       time.Duration
	refresh          refreshAccessTokenFunc

	mu     sync.Mutex
	loaded bool
	token  *AccessToken
}

type refreshAccessTokenFunc func(
	refreshURL string,
	refreshToken string,
	timeout time.Duration,
	transportOptions httpclient.TransportOptions,
	logger *slog.Logger,
) (*AccessToken, error)

type TokenSourceOptions struct {
	PAT              string
	RefreshURL       string
	Timeout          time.Duration
	TransportOptions httpclient.TransportOptions
	Logger           *slog.Logger
	ExpirySkew       time.Duration
	Refresh          refreshAccessTokenFunc
}

func NewTokenSource(cfg config.Hook, opts TokenSourceOptions) *TokenSource {
	skew := opts.ExpirySkew
	if skew <= 0 {
		skew = DefaultAccessTokenExpirySkew
	}
	refresh := opts.Refresh
	if refresh == nil {
		refresh = RefreshAccessToken
	}

	return &TokenSource{
		cfg:              cfg,
		pat:              strings.TrimSpace(opts.PAT),
		refreshURL:       strings.TrimSpace(opts.RefreshURL),
		timeout:          opts.Timeout,
		transportOptions: opts.TransportOptions,
		logger:           loggerOrDiscard(opts.Logger),
		expirySkew:       skew,
		refresh:          refresh,
	}
}

func (s *TokenSource) Token(ctx context.Context) (string, error) {
	return s.currentToken(ctx, false, "")
}

func (s *TokenSource) Refresh(ctx context.Context, previousToken string) (string, error) {
	return s.currentToken(ctx, true, previousToken)
}

func (s *TokenSource) Refreshable() bool {
	return s != nil && s.pat == ""
}

func (s *TokenSource) Security(ctx context.Context) (kkComps.Security, error) {
	token, err := s.Token(ctx)
	if err != nil {
		return kkComps.Security{}, err
	}
	return kkComps.Security{
		PersonalAccessToken: &token,
	}, nil
}

func (s *TokenSource) currentToken(ctx context.Context, forceRefresh bool, previousToken string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("konnect token source is not configured")
	}
	if err := ctxErr(ctx); err != nil {
		return "", err
	}
	if s.pat != "" {
		if forceRefresh {
			return "", ErrTokenRefreshUnsupported
		}
		return s.pat, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := ctxErr(ctx); err != nil {
		return "", err
	}

	if err := s.loadOnce(); err != nil {
		return "", err
	}
	if s.token == nil || s.token.Token == nil || strings.TrimSpace(s.token.Token.AuthToken) == "" {
		return "", fmt.Errorf("konnect access token is empty")
	}

	if forceRefresh && previousToken != "" && s.token.Token.AuthToken != previousToken &&
		!s.token.expiresWithin(s.expirySkew) {
		return s.token.Token.AuthToken, nil
	}

	if forceRefresh || s.token.expiresWithin(s.expirySkew) {
		if strings.TrimSpace(s.token.Token.RefreshToken) == "" {
			return "", fmt.Errorf("konnect refresh token is empty")
		}
		if strings.TrimSpace(s.refreshURL) == "" {
			return "", fmt.Errorf("konnect refresh URL is empty")
		}

		s.logger.Info("Token expired or near expiry, refreshing", "refresh_url", s.refreshURL)
		refreshed, err := s.refresh(
			s.refreshURL,
			s.token.Token.RefreshToken,
			s.timeout,
			s.transportOptions,
			s.logger,
		)
		if err != nil {
			return "", fmt.Errorf("refresh Konnect access token: %w", err)
		}
		if refreshed == nil || refreshed.Token == nil || strings.TrimSpace(refreshed.Token.AuthToken) == "" {
			return "", fmt.Errorf("refresh Konnect access token: empty token response")
		}

		if err := saveAccessTokenToDisk(credentialFilePath(s.cfg), refreshed); err != nil {
			return "", fmt.Errorf("save refreshed Konnect access token: %w", err)
		}
		s.token = refreshed
	}

	return s.token.Token.AuthToken, nil
}

func (s *TokenSource) loadOnce() error {
	if s.loaded {
		return nil
	}
	if s.cfg == nil {
		return fmt.Errorf("konnect config is not available")
	}

	creds, err := loadAccessTokenFromDisk(credentialFilePath(s.cfg))
	if err != nil {
		return err
	}
	s.token = creds
	s.loaded = true
	return nil
}

func credentialFilePath(cfg config.Hook) string {
	profile := cfg.GetProfile()
	cfgPath := filepath.Dir(cfg.GetPath())
	return filepath.Join(cfgPath, getCredentialFileName(profile))
}

func (t *AccessToken) expiresWithin(skew time.Duration) bool {
	if t == nil || t.Token == nil {
		return true
	}
	expiresAt := t.ReceivedAt.Add(time.Duration(t.Token.ExpiresAfter) * time.Second)
	return !time.Now().Add(skew).Before(expiresAt)
}

func ctxErr(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}

func loggerOrDiscard(logger *slog.Logger) *slog.Logger {
	if logger != nil {
		return logger
	}
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
