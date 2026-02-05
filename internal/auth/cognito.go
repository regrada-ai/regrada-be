package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

// Ensure CognitoService implements Service interface at compile time
var _ Service = (*CognitoService)(nil)

type CognitoService struct {
	client        *cognitoidentityprovider.Client
	userPoolID    string
	clientID      string
	clientSecret  string
	region        string
	cognitoDomain string
	jwkCache      *jwk.Cache
}

func NewCognitoService(region, userPoolID, clientID, clientSecret, cognitoDomain string) (*CognitoService, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := cognitoidentityprovider.NewFromConfig(cfg)

	// Set up JWK cache for token validation
	jwksURL := fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s/.well-known/jwks.json", region, userPoolID)
	cache := jwk.NewCache(context.Background())
	if err := cache.Register(jwksURL); err != nil {
		return nil, fmt.Errorf("failed to register JWKS URL: %w", err)
	}

	return &CognitoService{
		client:        client,
		userPoolID:    userPoolID,
		clientID:      clientID,
		clientSecret:  clientSecret,
		region:        region,
		cognitoDomain: cognitoDomain,
		jwkCache:      cache,
	}, nil
}

// SignUp creates a new user in Cognito
func (s *CognitoService) SignUp(ctx context.Context, email, password, name string) (*SignUpResult, error) {
	input := &cognitoidentityprovider.SignUpInput{
		ClientId: aws.String(s.clientID),
		Username: aws.String(email),
		Password: aws.String(password),
		UserAttributes: []types.AttributeType{
			{
				Name:  aws.String("email"),
				Value: aws.String(email),
			},
			{
				Name:  aws.String("name"),
				Value: aws.String(name),
			},
		},
	}

	// Add secret hash if client secret is configured
	if s.clientSecret != "" {
		secretHash := computeSecretHash(email, s.clientID, s.clientSecret)
		input.SecretHash = aws.String(secretHash)
	}

	result, err := s.client.SignUp(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("signup failed: %w", err)
	}

	return &SignUpResult{
		UserSub:       aws.ToString(result.UserSub),
		UserConfirmed: result.UserConfirmed,
	}, nil
}

// ConfirmSignUp confirms a user's email with verification code
func (s *CognitoService) ConfirmSignUp(ctx context.Context, email, code string) error {
	input := &cognitoidentityprovider.ConfirmSignUpInput{
		ClientId:         aws.String(s.clientID),
		Username:         aws.String(email),
		ConfirmationCode: aws.String(code),
	}

	if s.clientSecret != "" {
		secretHash := computeSecretHash(email, s.clientID, s.clientSecret)
		input.SecretHash = aws.String(secretHash)
	}

	_, err := s.client.ConfirmSignUp(ctx, input)
	if err != nil {
		return fmt.Errorf("confirmation failed: %w", err)
	}

	return nil
}

// SignIn authenticates a user and returns tokens
func (s *CognitoService) SignIn(ctx context.Context, email, password string) (*AuthTokens, error) {
	input := &cognitoidentityprovider.InitiateAuthInput{
		ClientId: aws.String(s.clientID),
		AuthFlow: types.AuthFlowTypeUserPasswordAuth,
		AuthParameters: map[string]string{
			"USERNAME": email,
			"PASSWORD": password,
		},
	}

	if s.clientSecret != "" {
		secretHash := computeSecretHash(email, s.clientID, s.clientSecret)
		input.AuthParameters["SECRET_HASH"] = secretHash
	}

	result, err := s.client.InitiateAuth(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("signin failed: %w", err)
	}

	if result.ChallengeName != "" {
		return nil, fmt.Errorf("challenge required: %s", result.ChallengeName)
	}

	return &AuthTokens{
		AccessToken:  aws.ToString(result.AuthenticationResult.AccessToken),
		IDToken:      aws.ToString(result.AuthenticationResult.IdToken),
		RefreshToken: aws.ToString(result.AuthenticationResult.RefreshToken),
		ExpiresIn:    result.AuthenticationResult.ExpiresIn,
	}, nil
}

// RefreshToken refreshes the access token using a refresh token
func (s *CognitoService) RefreshToken(ctx context.Context, refreshToken string) (*AuthTokens, error) {
	input := &cognitoidentityprovider.InitiateAuthInput{
		ClientId: aws.String(s.clientID),
		AuthFlow: types.AuthFlowTypeRefreshTokenAuth,
		AuthParameters: map[string]string{
			"REFRESH_TOKEN": refreshToken,
		},
	}

	result, err := s.client.InitiateAuth(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("token refresh failed: %w", err)
	}

	return &AuthTokens{
		AccessToken: aws.ToString(result.AuthenticationResult.AccessToken),
		IDToken:     aws.ToString(result.AuthenticationResult.IdToken),
		ExpiresIn:   result.AuthenticationResult.ExpiresIn,
	}, nil
}

// GetUser retrieves user information from an access token
func (s *CognitoService) GetUser(ctx context.Context, accessToken string) (*UserInfo, error) {
	input := &cognitoidentityprovider.GetUserInput{
		AccessToken: aws.String(accessToken),
	}

	result, err := s.client.GetUser(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("get user failed: %w", err)
	}

	userInfo := &UserInfo{
		Username: aws.ToString(result.Username),
	}

	for _, attr := range result.UserAttributes {
		switch aws.ToString(attr.Name) {
		case "sub":
			userInfo.Sub = aws.ToString(attr.Value)
		case "email":
			userInfo.Email = aws.ToString(attr.Value)
		case "email_verified":
			userInfo.EmailVerified = aws.ToString(attr.Value) == "true"
		case "name":
			userInfo.Name = aws.ToString(attr.Value)
		case "picture":
			userInfo.Picture = aws.ToString(attr.Value)
		}
	}

	return userInfo, nil
}

// SignOut globally signs out a user
func (s *CognitoService) SignOut(ctx context.Context, accessToken string) error {
	input := &cognitoidentityprovider.GlobalSignOutInput{
		AccessToken: aws.String(accessToken),
	}

	_, err := s.client.GlobalSignOut(ctx, input)
	if err != nil {
		return fmt.Errorf("signout failed: %w", err)
	}

	return nil
}

// ValidateIDToken validates a Cognito ID token and extracts user info
// This is used for OAuth flows (Google sign-in) where the frontend has the tokens
func (s *CognitoService) ValidateIDToken(ctx context.Context, idToken string) (*UserInfo, *AuthTokens, error) {
	jwksURL := fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s/.well-known/jwks.json", s.region, s.userPoolID)

	// Fetch the key set from cache
	keySet, err := s.jwkCache.Get(ctx, jwksURL)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch JWKS: %w", err)
	}

	// Parse and validate the token
	token, err := jwt.Parse([]byte(idToken),
		jwt.WithKeySet(keySet),
		jwt.WithValidate(true),
		jwt.WithIssuer(fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s", s.region, s.userPoolID)),
		jwt.WithAudience(s.clientID),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid ID token: %w", err)
	}

	// Extract claims
	claims := token.PrivateClaims()

	userInfo := &UserInfo{
		Sub: token.Subject(),
	}

	if email, ok := claims["email"].(string); ok {
		userInfo.Email = email
	}
	if emailVerified, ok := claims["email_verified"].(bool); ok {
		userInfo.EmailVerified = emailVerified
	}

	// Try to get name from various possible claim locations
	// Google/Cognito federated users might have name in different places
	if name, ok := claims["name"].(string); ok && name != "" {
		userInfo.Name = name
	} else {
		// Fall back to constructing name from given_name and family_name
		var givenName, familyName string
		if gn, ok := claims["given_name"].(string); ok {
			givenName = gn
		}
		if fn, ok := claims["family_name"].(string); ok {
			familyName = fn
		}
		if givenName != "" || familyName != "" {
			userInfo.Name = strings.TrimSpace(givenName + " " + familyName)
		}
	}

	// Try to get picture from various possible claim locations
	if picture, ok := claims["picture"].(string); ok && picture != "" {
		userInfo.Picture = picture
	}

	if username, ok := claims["cognito:username"].(string); ok {
		userInfo.Username = username
	}

	// For OAuth flows, the frontend already has the tokens from Cognito
	// We return nil for AuthTokens since the frontend should pass them separately
	return userInfo, nil, nil
}

// ExchangeCodeForTokens exchanges an OAuth authorization code for tokens
// This is the secure server-side flow where the client secret is kept on the backend
func (s *CognitoService) ExchangeCodeForTokens(ctx context.Context, code, redirectURI string) (*AuthTokens, error) {
	tokenURL := fmt.Sprintf("https://%s/oauth2/token", s.cognitoDomain)

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("client_id", s.clientID)
	data.Set("client_secret", s.clientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err == nil {
			return nil, fmt.Errorf("token exchange failed: %s - %s", errResp.Error, errResp.ErrorDescription)
		}
		return nil, fmt.Errorf("token exchange failed with status: %d", resp.StatusCode)
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		IDToken      string `json:"id_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int32  `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	return &AuthTokens{
		AccessToken:  tokenResp.AccessToken,
		IDToken:      tokenResp.IDToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresIn:    tokenResp.ExpiresIn,
	}, nil
}
