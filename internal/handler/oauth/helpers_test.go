package oauthhandler

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/sessions"

	"github.com/c-premus/documcp/internal/auth/oauth"
	"github.com/c-premus/documcp/internal/config"
	"github.com/c-premus/documcp/internal/model"
)

// ---------------------------------------------------------------------------
// Mock OAuthRepo — callback-based mock implementing oauth.OAuthRepo
// ---------------------------------------------------------------------------

type mockOAuthRepo struct {
	CreateClientFunc                      func(ctx context.Context, client *model.OAuthClient) error
	FindClientByClientIDFunc              func(ctx context.Context, clientID string) (*model.OAuthClient, error)
	FindClientByIDFunc                    func(ctx context.Context, id int64) (*model.OAuthClient, error)
	TouchClientLastUsedFunc               func(ctx context.Context, clientID int64) error
	CreateAuthorizationCodeFunc           func(ctx context.Context, code *model.OAuthAuthorizationCode) error
	FindAuthorizationCodeByCodeFunc       func(ctx context.Context, codeHash string) (*model.OAuthAuthorizationCode, error)
	RevokeAuthorizationCodeFunc           func(ctx context.Context, id int64) error
	CreateAccessTokenFunc                 func(ctx context.Context, token *model.OAuthAccessToken) error
	FindAccessTokenByIDFunc               func(ctx context.Context, id int64) (*model.OAuthAccessToken, error)
	FindAccessTokenByTokenFunc            func(ctx context.Context, tokenHash string) (*model.OAuthAccessToken, error)
	RevokeAccessTokenFunc                 func(ctx context.Context, id int64) error
	CreateRefreshTokenFunc                func(ctx context.Context, token *model.OAuthRefreshToken) error
	FindRefreshTokenByTokenFunc           func(ctx context.Context, tokenHash string) (*model.OAuthRefreshToken, error)
	RevokeRefreshTokenFunc                func(ctx context.Context, id int64) error
	RevokeRefreshTokenByAccessTokenIDFunc func(ctx context.Context, accessTokenID int64) error
	CreateDeviceCodeFunc                  func(ctx context.Context, dc *model.OAuthDeviceCode) error
	FindDeviceCodeByDeviceCodeFunc        func(ctx context.Context, deviceCodeHash string) (*model.OAuthDeviceCode, error)
	FindDeviceCodeByUserCodeFunc          func(ctx context.Context, userCode string) (*model.OAuthDeviceCode, error)
	UpdateDeviceCodeStatusFunc            func(ctx context.Context, id int64, status string, userID *int64) error
	UpdateDeviceCodeLastPolledFunc        func(ctx context.Context, id int64, interval int) error
	FindUserByIDFunc                      func(ctx context.Context, id int64) (*model.User, error)
}

func (m *mockOAuthRepo) CreateClient(ctx context.Context, client *model.OAuthClient) error {
	if m.CreateClientFunc != nil {
		return m.CreateClientFunc(ctx, client)
	}
	return nil
}

func (m *mockOAuthRepo) FindClientByClientID(ctx context.Context, clientID string) (*model.OAuthClient, error) {
	if m.FindClientByClientIDFunc != nil {
		return m.FindClientByClientIDFunc(ctx, clientID)
	}
	return nil, nil
}

func (m *mockOAuthRepo) FindClientByID(ctx context.Context, id int64) (*model.OAuthClient, error) {
	if m.FindClientByIDFunc != nil {
		return m.FindClientByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *mockOAuthRepo) TouchClientLastUsed(ctx context.Context, clientID int64) error {
	if m.TouchClientLastUsedFunc != nil {
		return m.TouchClientLastUsedFunc(ctx, clientID)
	}
	return nil
}

func (m *mockOAuthRepo) CreateAuthorizationCode(ctx context.Context, code *model.OAuthAuthorizationCode) error {
	if m.CreateAuthorizationCodeFunc != nil {
		return m.CreateAuthorizationCodeFunc(ctx, code)
	}
	return nil
}

func (m *mockOAuthRepo) FindAuthorizationCodeByCode(ctx context.Context, codeHash string) (*model.OAuthAuthorizationCode, error) {
	if m.FindAuthorizationCodeByCodeFunc != nil {
		return m.FindAuthorizationCodeByCodeFunc(ctx, codeHash)
	}
	return nil, nil
}

func (m *mockOAuthRepo) RevokeAuthorizationCode(ctx context.Context, id int64) error {
	if m.RevokeAuthorizationCodeFunc != nil {
		return m.RevokeAuthorizationCodeFunc(ctx, id)
	}
	return nil
}

func (m *mockOAuthRepo) CreateAccessToken(ctx context.Context, token *model.OAuthAccessToken) error {
	if m.CreateAccessTokenFunc != nil {
		return m.CreateAccessTokenFunc(ctx, token)
	}
	return nil
}

func (m *mockOAuthRepo) FindAccessTokenByID(ctx context.Context, id int64) (*model.OAuthAccessToken, error) {
	if m.FindAccessTokenByIDFunc != nil {
		return m.FindAccessTokenByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *mockOAuthRepo) FindAccessTokenByToken(ctx context.Context, tokenHash string) (*model.OAuthAccessToken, error) {
	if m.FindAccessTokenByTokenFunc != nil {
		return m.FindAccessTokenByTokenFunc(ctx, tokenHash)
	}
	return nil, nil
}

func (m *mockOAuthRepo) RevokeAccessToken(ctx context.Context, id int64) error {
	if m.RevokeAccessTokenFunc != nil {
		return m.RevokeAccessTokenFunc(ctx, id)
	}
	return nil
}

func (m *mockOAuthRepo) RevokeTokenPair(_ context.Context, _, _ int64) error {
	return nil
}

func (m *mockOAuthRepo) CreateRefreshToken(ctx context.Context, token *model.OAuthRefreshToken) error {
	if m.CreateRefreshTokenFunc != nil {
		return m.CreateRefreshTokenFunc(ctx, token)
	}
	return nil
}

func (m *mockOAuthRepo) FindRefreshTokenByToken(ctx context.Context, tokenHash string) (*model.OAuthRefreshToken, error) {
	if m.FindRefreshTokenByTokenFunc != nil {
		return m.FindRefreshTokenByTokenFunc(ctx, tokenHash)
	}
	return nil, nil
}

func (m *mockOAuthRepo) RevokeRefreshToken(ctx context.Context, id int64) error {
	if m.RevokeRefreshTokenFunc != nil {
		return m.RevokeRefreshTokenFunc(ctx, id)
	}
	return nil
}

func (m *mockOAuthRepo) RevokeRefreshTokenByAccessTokenID(ctx context.Context, accessTokenID int64) error {
	if m.RevokeRefreshTokenByAccessTokenIDFunc != nil {
		return m.RevokeRefreshTokenByAccessTokenIDFunc(ctx, accessTokenID)
	}
	return nil
}

func (m *mockOAuthRepo) CreateDeviceCode(ctx context.Context, dc *model.OAuthDeviceCode) error {
	if m.CreateDeviceCodeFunc != nil {
		return m.CreateDeviceCodeFunc(ctx, dc)
	}
	return nil
}

func (m *mockOAuthRepo) FindDeviceCodeByDeviceCode(ctx context.Context, deviceCodeHash string) (*model.OAuthDeviceCode, error) {
	if m.FindDeviceCodeByDeviceCodeFunc != nil {
		return m.FindDeviceCodeByDeviceCodeFunc(ctx, deviceCodeHash)
	}
	return nil, nil
}

func (m *mockOAuthRepo) FindDeviceCodeByUserCode(ctx context.Context, userCode string) (*model.OAuthDeviceCode, error) {
	if m.FindDeviceCodeByUserCodeFunc != nil {
		return m.FindDeviceCodeByUserCodeFunc(ctx, userCode)
	}
	return nil, nil
}

func (m *mockOAuthRepo) UpdateDeviceCodeStatus(ctx context.Context, id int64, status string, userID *int64) error {
	if m.UpdateDeviceCodeStatusFunc != nil {
		return m.UpdateDeviceCodeStatusFunc(ctx, id, status, userID)
	}
	return nil
}

func (m *mockOAuthRepo) UpdateDeviceCodeLastPolled(ctx context.Context, id int64, interval int) error {
	if m.UpdateDeviceCodeLastPolledFunc != nil {
		return m.UpdateDeviceCodeLastPolledFunc(ctx, id, interval)
	}
	return nil
}

func (m *mockOAuthRepo) FindUserByID(ctx context.Context, id int64) (*model.User, error) {
	if m.FindUserByIDFunc != nil {
		return m.FindUserByIDFunc(ctx, id)
	}
	return nil, nil
}

// ---------------------------------------------------------------------------
// Mock session store
// ---------------------------------------------------------------------------

type mockSessionStore struct {
	session *sessions.Session
}

func newMockSessionStore() *mockSessionStore {
	store := &mockSessionStore{}
	store.session = sessions.NewSession(store, sessionName)
	store.session.Values = make(map[any]any)
	return store
}

func (m *mockSessionStore) Get(_ *http.Request, name string) (*sessions.Session, error) {
	if m.session.Values == nil {
		m.session.Values = make(map[any]any)
	}
	return m.session, nil
}

func (m *mockSessionStore) New(_ *http.Request, name string) (*sessions.Session, error) {
	return m.session, nil
}

func (m *mockSessionStore) Save(_ *http.Request, _ http.ResponseWriter, _ *sessions.Session) error {
	return nil
}

// ---------------------------------------------------------------------------
// Test fixture helpers
// ---------------------------------------------------------------------------

func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func defaultOAuthConfig() config.OAuthConfig {
	return config.OAuthConfig{
		AuthCodeLifetime:        10 * time.Minute,
		AccessTokenLifetime:     1 * time.Hour,
		RefreshTokenLifetime:    30 * 24 * time.Hour,
		DeviceCodeLifetime:      15 * time.Minute,
		DeviceCodeInterval:      5 * time.Second,
		RegistrationEnabled:     true,
		RegistrationRequireAuth: false,
	}
}

func newHandlerWithRepo(repo *mockOAuthRepo) (*Handler, *mockSessionStore) {
	return newHandlerWithRepoAndConfig(repo, defaultOAuthConfig())
}

func newHandlerWithRepoAndConfig(repo *mockOAuthRepo, oauthCfg config.OAuthConfig) (*Handler, *mockSessionStore) {
	store := newMockSessionStore()
	svc := oauth.NewService(repo, oauthCfg, "https://example.com", discardLogger())
	h := New(Config{
		Service:      svc,
		SessionStore: store,
		OAuthCfg:     oauthCfg,
		AppURL:       "https://example.com",
		Logger:       discardLogger(),
	})
	return h, store
}

func decodeOAuthJSON(t *testing.T, body io.Reader) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.NewDecoder(body).Decode(&result); err != nil {
		t.Fatalf("decoding JSON response: %v", err)
	}
	return result
}
