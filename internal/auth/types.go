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
	ID             string
	Sub            string
	Username       string
	Email          string
	Name           string
	Picture        string
	OrganizationID string
}

// Service defines the interface for authentication operations
type Service interface {
	SignUp(ctx context.Context, email, password, name, organizationID string) (*SignUpResult, error)
	ConfirmSignUp(ctx context.Context, email, code string) error
	SignIn(ctx context.Context, email, password string) (*AuthTokens, error)
	RefreshToken(ctx context.Context, refreshToken string) (*AuthTokens, error)
	GetUser(ctx context.Context, accessToken string) (*UserInfo, error)
	AdminUpdateUserOrganization(ctx context.Context, username, organizationID string) error
	UpdateUserOrganization(ctx context.Context, accessToken, organizationID string) error
	SignOut(ctx context.Context, accessToken string) error
}
