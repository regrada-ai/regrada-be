package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"github.com/aws/smithy-go"
)

// Ensure CognitoService implements Service interface at compile time
var _ Service = (*CognitoService)(nil)

type CognitoService struct {
	client       *cognitoidentityprovider.Client
	userPoolID   string
	clientID     string
	clientSecret string
}

func NewCognitoService(region, userPoolID, clientID, clientSecret string) (*CognitoService, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := cognitoidentityprovider.NewFromConfig(cfg)

	return &CognitoService{
		client:       client,
		userPoolID:   userPoolID,
		clientID:     clientID,
		clientSecret: clientSecret,
	}, nil
}

// SignUp creates a new user in Cognito
func (s *CognitoService) SignUp(ctx context.Context, email, password, name, organizationID string) (*SignUpResult, error) {
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

	if organizationID != "" {
		input.UserAttributes = append(input.UserAttributes, types.AttributeType{
			Name:  aws.String("custom:organization_id"),
			Value: aws.String(organizationID),
		})
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
		case "name":
			userInfo.Name = aws.ToString(attr.Value)
		case "custom:organization_id":
			userInfo.OrganizationID = aws.ToString(attr.Value)
		case "picture":
			userInfo.Picture = aws.ToString(attr.Value)
		}
	}

	return userInfo, nil
}

// AdminUpdateUserOrganization sets the custom organization_id attribute for a user by username.
func (s *CognitoService) AdminUpdateUserOrganization(ctx context.Context, username, organizationID string) error {
	input := &cognitoidentityprovider.AdminUpdateUserAttributesInput{
		UserPoolId: aws.String(s.userPoolID),
		Username:   aws.String(username),
		UserAttributes: []types.AttributeType{
			{
				Name:  aws.String("custom:organization_id"),
				Value: aws.String(organizationID),
			},
		},
	}

	if _, err := s.client.AdminUpdateUserAttributes(ctx, input); err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "UserNotFoundException" {
			return ErrUserNotFound
		}
		return fmt.Errorf("admin update user attributes failed: %w", err)
	}

	return nil
}

// UpdateUserOrganization sets the custom organization_id attribute for the current user.
func (s *CognitoService) UpdateUserOrganization(ctx context.Context, accessToken, organizationID string) error {
	input := &cognitoidentityprovider.UpdateUserAttributesInput{
		AccessToken: aws.String(accessToken),
		UserAttributes: []types.AttributeType{
			{
				Name:  aws.String("custom:organization_id"),
				Value: aws.String(organizationID),
			},
		},
	}

	if _, err := s.client.UpdateUserAttributes(ctx, input); err != nil {
		return fmt.Errorf("update user attributes failed: %w", err)
	}

	return nil
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
