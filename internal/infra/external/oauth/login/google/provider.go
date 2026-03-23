package google

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/oauth2"
	googleoauth2 "golang.org/x/oauth2/google"

	repoexternal "github.com/eannchen/go-backend-architecture/internal/repository/external/oauth"
)

const userInfoURL = "https://www.googleapis.com/oauth2/v2/userinfo"

// Provider implements OAuthProvider for Google OAuth2 login (identity only).
type Provider struct {
	cfg *oauth2.Config
}

// Config holds settings needed to construct the Google OAuth2 login provider.
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// NewProvider returns a Google login provider (openid, email).
func NewProvider(cfg Config) *Provider {
	return &Provider{
		cfg: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       []string{"openid", "email"},
			Endpoint:     googleoauth2.Endpoint,
		},
	}
}

func (p *Provider) Name() string {
	return "google"
}

func (p *Provider) AuthCodeURL(state string) string {
	return p.cfg.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

func (p *Provider) Exchange(ctx context.Context, code string) (repoexternal.OAuthUserInfo, error) {
	token, err := p.cfg.Exchange(ctx, code)
	if err != nil {
		return repoexternal.OAuthUserInfo{}, fmt.Errorf("exchange oauth code: %w", err)
	}

	client := p.cfg.Client(ctx, token)
	resp, err := client.Get(userInfoURL)
	if err != nil {
		return repoexternal.OAuthUserInfo{}, fmt.Errorf("fetch google user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return repoexternal.OAuthUserInfo{}, fmt.Errorf("google user info returned %d: %s", resp.StatusCode, body)
	}

	var info struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return repoexternal.OAuthUserInfo{}, fmt.Errorf("decode google user info: %w", err)
	}
	if info.Email == "" {
		return repoexternal.OAuthUserInfo{}, fmt.Errorf("google user info missing email")
	}

	return repoexternal.OAuthUserInfo{
		ProviderUserID: info.ID,
		Email:          info.Email,
	}, nil
}

var _ repoexternal.OAuthProvider = (*Provider)(nil)
