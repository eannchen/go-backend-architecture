package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	repoexternal "github.com/eannchen/go-backend-architecture/internal/repository/external"
)

const googleUserInfoURL = "https://www.googleapis.com/oauth2/v2/userinfo"

// GoogleProvider implements OAuthProvider for Google OAuth2.
type GoogleProvider struct {
	cfg *oauth2.Config
}

// GoogleConfig holds settings needed to construct the Google OAuth2 provider.
type GoogleConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

func NewGoogleProvider(cfg GoogleConfig) *GoogleProvider {
	return &GoogleProvider{
		cfg: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       []string{"openid", "email"},
			Endpoint:     google.Endpoint,
		},
	}
}

func (p *GoogleProvider) Name() string {
	return "google"
}

func (p *GoogleProvider) AuthCodeURL(state string) string {
	return p.cfg.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

func (p *GoogleProvider) Exchange(ctx context.Context, code string) (repoexternal.OAuthUserInfo, error) {
	token, err := p.cfg.Exchange(ctx, code)
	if err != nil {
		return repoexternal.OAuthUserInfo{}, fmt.Errorf("exchange oauth code: %w", err)
	}

	client := p.cfg.Client(ctx, token)
	resp, err := client.Get(googleUserInfoURL)
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

var _ repoexternal.OAuthProvider = (*GoogleProvider)(nil)
