package auth

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
	Sub            string
	Username       string
	Email          string
	Name           string
	Picture        string
	OrganizationID string
}
