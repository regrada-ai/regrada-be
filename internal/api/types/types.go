package types

import "time"

// Error represents an API error response
type Error struct {
	Code    string `json:"code" example:"INVALID_REQUEST"`
	Message string `json:"message" example:"Invalid request parameters"`
}

// ErrorResponse wraps an error
type ErrorResponse struct {
	Error Error `json:"error"`
}

// SignUpRequest represents a user signup request
type SignUpRequest struct {
	Email              string `json:"email" binding:"required,email" example:"user@example.com"`
	Password           string `json:"password" binding:"required,min=8" example:"password123"`
	Name               string `json:"name" binding:"required" example:"John Doe"`
	CreateOrganization bool   `json:"create_organization,omitempty" example:"true"`
	OrganizationName   string `json:"organization_name,omitempty" example:"My Company"`
	InviteToken        string `json:"invite_token,omitempty" example:"abc123token"`
}

// SignUpResponse represents the signup response
type SignUpResponse struct {
	Success        bool   `json:"success" example:"true"`
	UserConfirmed  bool   `json:"user_confirmed" example:"false"`
	OrganizationID string `json:"organization_id,omitempty" example:"123e4567-e89b-12d3-a456-426614174000"`
	Message        string `json:"message" example:"Sign up successful. Please check your email for verification code."`
}

// SignInRequest represents a signin request
type SignInRequest struct {
	Email    string `json:"email" binding:"required,email" example:"user@example.com"`
	Password string `json:"password" binding:"required" example:"password123"`
}

// SignInResponse represents the signin response
type SignInResponse struct {
	Success bool   `json:"success" example:"true"`
	Message string `json:"message" example:"Sign in successful"`
}

// CreateOrganizationRequest represents a request to create an organization
type CreateOrganizationRequest struct {
	Name string `json:"name" binding:"required" example:"My Company"`
	Slug string `json:"slug" binding:"required" example:"my-company"`
	Tier string `json:"tier" binding:"required,oneof=standard pro enterprise" example:"standard"`
}

// Organization represents an organization
type Organization struct {
	ID            string    `json:"id" example:"123e4567-e89b-12d3-a456-426614174000"`
	Name          string    `json:"name" example:"My Company"`
	Slug          string    `json:"slug" example:"my-company"`
	Tier          string    `json:"tier" example:"standard"`
	GitHubOrgID   *int64    `json:"github_org_id,omitempty"`
	GitHubOrgName string    `json:"github_org_name,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// CreateAPIKeyRequest represents a request to create an API key
type CreateAPIKeyRequest struct {
	Name      string     `json:"name" binding:"required" example:"Production API Key"`
	Tier      string     `json:"tier" binding:"required" example:"standard"`
	Scopes    []string   `json:"scopes,omitempty" example:"traces:write,tests:write"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// APIKeyResponse represents an API key response
type APIKeyResponse struct {
	ID           string     `json:"id" example:"123e4567-e89b-12d3-a456-426614174000"`
	Name         string     `json:"name" example:"Production API Key"`
	KeyPrefix    string     `json:"key_prefix" example:"rg_live_abc123"`
	Tier         string     `json:"tier" example:"standard"`
	Scopes       []string   `json:"scopes" example:"traces:write,tests:write"`
	RateLimitRPM int        `json:"rate_limit_rpm" example:"100"`
	LastUsedAt   *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	RevokedAt    *time.Time `json:"revoked_at,omitempty"`
}

// CreateAPIKeyResponse includes the secret (only returned once)
type CreateAPIKeyResponse struct {
	APIKey APIKeyResponse `json:"api_key"`
	Secret string         `json:"secret" example:"rg_live_abcdefghijklmnopqrstuvwxyz123456"`
}

// CreateInviteRequest represents a request to create an invite
type CreateInviteRequest struct {
	Email          string `json:"email" binding:"required,email" example:"user@example.com"`
	Role           string `json:"role" binding:"required,oneof=admin user readonly-user" example:"user"`
	ExpiresInHours int    `json:"expires_in_hours,omitempty" example:"168"`
}

// InviteResponse represents an invite
type InviteResponse struct {
	ID             string     `json:"id" example:"123e4567-e89b-12d3-a456-426614174000"`
	Email          string     `json:"email" example:"user@example.com"`
	Role           string     `json:"role" example:"user"`
	Token          string     `json:"token" example:"abc123token"`
	OrganizationID string     `json:"organization_id,omitempty"`
	ExpiresAt      time.Time  `json:"expires_at"`
	AcceptedAt     *time.Time `json:"accepted_at,omitempty"`
	RevokedAt      *time.Time `json:"revoked_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// UserResponse represents a user
type UserResponse struct {
	ID        string    `json:"id" example:"123e4567-e89b-12d3-a456-426614174000"`
	Email     string    `json:"email" example:"user@example.com"`
	Name      string    `json:"name" example:"John Doe"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SuccessResponse represents a generic success response
type SuccessResponse struct {
	Success bool `json:"success" example:"true"`
}
