package auth

import "context"

type SignUpResult struct {
	UserSub       string
	UserConfirmed bool
}

type AuthTokens struct {
	AccessToken  string
	IDToken      string
	RefreshToken string
	ExpiresIn    int32
}

type UserInfo struct {
	ID            string
	Sub           string
	Username      string
	Email         string
	EmailVerified bool
	Name          string
	Picture       string
}

// Service defines the interface for authentication operations
type Service interface {
	SignUp(ctx context.Context, email, password, name string) (*SignUpResult, error)
	ConfirmSignUp(ctx context.Context, email, code string) error
	SignIn(ctx context.Context, email, password string) (*AuthTokens, error)
	RefreshToken(ctx context.Context, refreshToken string) (*AuthTokens, error)
	GetUser(ctx context.Context, accessToken string) (*UserInfo, error)
	SignOut(ctx context.Context, accessToken string) error
	// ValidateIDToken validates a Cognito ID token (from OAuth flow) and returns user info and tokens
	ValidateIDToken(ctx context.Context, idToken string) (*UserInfo, *AuthTokens, error)
	// ExchangeCodeForTokens exchanges an OAuth authorization code for tokens (server-side flow)
	ExchangeCodeForTokens(ctx context.Context, code, redirectURI string) (*AuthTokens, error)
}
