package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const mockAuthFile = "data/mock_auth_data.json"

// Ensure MockService implements Service interface at compile time
var _ Service = (*MockService)(nil)

// MockService is a mock authentication service for local development
// It simulates Cognito behavior without making actual AWS calls
type MockService struct {
	users         map[string]*mockUser // email -> user
	tokens        map[string]*mockUser // accessToken -> user
	refreshTokens map[string]*mockUser // refreshToken -> user
	mu            sync.RWMutex
}

type mockUser struct {
	Sub          string
	Email        string
	Password     string
	Name         string
	Picture      string
	Confirmed    bool
	ConfirmCode  string
	AccessToken  string
	RefreshToken string
	IDToken      string
	TokenExpiry  time.Time
}

func NewMockService() *MockService {
	s := &MockService{
		users:         make(map[string]*mockUser),
		tokens:        make(map[string]*mockUser),
		refreshTokens: make(map[string]*mockUser),
	}
	s.load()
	return s
}

// SignUp creates a new mock user
func (s *MockService) SignUp(ctx context.Context, email, password, name string) (*SignUpResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if user already exists
	if _, exists := s.users[email]; exists {
		return nil, fmt.Errorf("user already exists")
	}

	// Generate a confirmation code
	confirmCode := generateRandomCode(6)

	user := &mockUser{
		Sub:         uuid.New().String(),
		Email:       email,
		Password:    password,
		Name:        name,
		Confirmed:   false, // Auto-confirm in mock mode for easier testing
		ConfirmCode: confirmCode,
	}

	s.users[email] = user
	s.save()

	fmt.Printf("[MOCK AUTH] User signed up: %s (code: %s)\n", email, confirmCode)

	return &SignUpResult{
		UserSub:       user.Sub,
		UserConfirmed: true, // Auto-confirm for easier local testing
	}, nil
}

// ConfirmSignUp confirms a user's email (mock always succeeds)
func (s *MockService) ConfirmSignUp(ctx context.Context, email, code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.users[email]
	if !exists {
		return fmt.Errorf("user not found")
	}

	// In mock mode, accept any code or auto-confirm
	user.Confirmed = true
	s.save()
	fmt.Printf("[MOCK AUTH] User confirmed: %s\n", email)

	return nil
}

// SignIn authenticates a user and returns mock tokens
func (s *MockService) SignIn(ctx context.Context, email, password string) (*AuthTokens, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.users[email]
	if !exists {
		return nil, fmt.Errorf("user not found")
	}

	// In production Cognito, we'd verify password
	// For mock, we'll do a simple check
	if user.Password != password {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Generate mock tokens
	accessToken := generateToken()
	refreshToken := generateToken()
	idToken := generateToken()

	user.AccessToken = accessToken
	user.RefreshToken = refreshToken
	user.IDToken = idToken
	user.TokenExpiry = time.Now().Add(1 * time.Hour)

	s.tokens[accessToken] = user
	s.refreshTokens[refreshToken] = user
	s.save()

	fmt.Printf("[MOCK AUTH] User signed in: %s\n", email)

	return &AuthTokens{
		AccessToken:  accessToken,
		IDToken:      idToken,
		RefreshToken: refreshToken,
		ExpiresIn:    3600, // 1 hour
	}, nil
}

// RefreshToken refreshes the access token using a refresh token
func (s *MockService) RefreshToken(ctx context.Context, refreshToken string) (*AuthTokens, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.refreshTokens[refreshToken]
	if !exists {
		return nil, fmt.Errorf("invalid refresh token")
	}

	// Generate new access token
	newAccessToken := generateToken()
	newIDToken := generateToken()

	// Clean up old access token
	delete(s.tokens, user.AccessToken)

	user.AccessToken = newAccessToken
	user.IDToken = newIDToken
	user.TokenExpiry = time.Now().Add(1 * time.Hour)

	s.tokens[newAccessToken] = user

	fmt.Printf("[MOCK AUTH] Token refreshed for: %s\n", user.Email)

	return &AuthTokens{
		AccessToken: newAccessToken,
		IDToken:     newIDToken,
		ExpiresIn:   3600,
	}, nil
}

// GetUser retrieves user information from an access token
func (s *MockService) GetUser(ctx context.Context, accessToken string) (*UserInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.tokens[accessToken]
	if !exists {
		return nil, fmt.Errorf("invalid access token")
	}

	// Check if token is expired
	if time.Now().After(user.TokenExpiry) {
		return nil, fmt.Errorf("token expired")
	}

	return &UserInfo{
		Sub:      user.Sub,
		Username: user.Email,
		Email:    user.Email,
		Name:     user.Name,
		Picture:  user.Picture,
	}, nil
}

// SignOut removes the user's tokens
func (s *MockService) SignOut(ctx context.Context, accessToken string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.tokens[accessToken]
	if !exists {
		return fmt.Errorf("invalid access token")
	}

	// Clean up tokens
	delete(s.tokens, user.AccessToken)
	delete(s.refreshTokens, user.RefreshToken)

	user.AccessToken = ""
	user.RefreshToken = ""
	user.IDToken = ""

	fmt.Printf("[MOCK AUTH] User signed out: %s\n", user.Email)

	return nil
}

// Helper functions

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func generateRandomCode(length int) string {
	const digits = "0123456789"
	b := make([]byte, length)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		b[i] = digits[n.Int64()]
	}
	return string(b)
}

// persistedData is the structure saved to disk
type persistedData struct {
	Users map[string]*mockUser `json:"users"`
}

// save persists mock auth data to disk (must be called with lock held)
func (s *MockService) save() {
	data := persistedData{Users: s.users}
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Printf("[MOCK AUTH] Failed to marshal data: %v\n", err)
		return
	}

	// Ensure data directory exists
	if err := os.MkdirAll("data", 0755); err != nil {
		fmt.Printf("[MOCK AUTH] Failed to create data directory: %v\n", err)
		return
	}

	if err := os.WriteFile(mockAuthFile, bytes, 0600); err != nil {
		fmt.Printf("[MOCK AUTH] Failed to save data: %v\n", err)
		return
	}
}

// load restores mock auth data from disk
func (s *MockService) load() {
	bytes, err := os.ReadFile(mockAuthFile)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Printf("[MOCK AUTH] Failed to read data: %v\n", err)
		}
		return
	}

	var data persistedData
	if err := json.Unmarshal(bytes, &data); err != nil {
		fmt.Printf("[MOCK AUTH] Failed to unmarshal data: %v\n", err)
		return
	}

	s.users = data.Users
	if s.users == nil {
		s.users = make(map[string]*mockUser)
	}

	fmt.Printf("[MOCK AUTH] Loaded %d users from %s\n", len(s.users), mockAuthFile)
}

// ExchangeCodeForTokens simulates OAuth code exchange for local development
// In mock mode, the "code" can be an email for easy testing, or a real code (which we'll use to generate a mock user)
func (s *MockService) ExchangeCodeForTokens(ctx context.Context, code, redirectURI string) (*AuthTokens, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// In mock mode, treat the code as an email if it contains "@", otherwise generate a mock email
	email := code
	name := "Mock User"

	if idx := strings.Index(email, "@"); idx > 0 {
		name = email[:idx]
	} else {
		// Real auth code from Cognito - generate a mock user
		email = fmt.Sprintf("mockuser_%s@example.com", code[:8])
		name = "Mock OAuth User"
	}

	user, exists := s.users[email]
	if !exists {
		// Auto-create user for mock OAuth
		user = &mockUser{
			Sub:       uuid.New().String(),
			Email:     email,
			Name:      name,
			Confirmed: true,
		}
		s.users[email] = user
	}

	// Generate tokens
	accessToken := generateToken()
	refreshToken := generateToken()
	idToken := generateToken()

	user.AccessToken = accessToken
	user.RefreshToken = refreshToken
	user.IDToken = idToken
	user.TokenExpiry = time.Now().Add(1 * time.Hour)

	s.tokens[accessToken] = user
	s.refreshTokens[refreshToken] = user
	s.save()

	fmt.Printf("[MOCK AUTH] OAuth code exchange: %s\n", email)

	return &AuthTokens{
		AccessToken:  accessToken,
		IDToken:      idToken,
		RefreshToken: refreshToken,
		ExpiresIn:    3600,
	}, nil
}

// ValidateIDToken simulates ID token validation for local development
// In mock mode, the "idToken" is treated as an email for easy testing
// This allows testing OAuth flows without real Cognito/Google integration
func (s *MockService) ValidateIDToken(ctx context.Context, idToken string) (*UserInfo, *AuthTokens, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// In mock mode, treat the idToken as an email for easy testing
	// Format: "email@example.com" or "email@example.com:Name"
	email := idToken
	name := ""

	// Check if name is provided after colon
	if idx := len(email) - 1; idx > 0 {
		for i := len(idToken) - 1; i >= 0; i-- {
			if idToken[i] == ':' {
				email = idToken[:i]
				name = idToken[i+1:]
				break
			}
		}
	}

	if name == "" {
		// Default name from email
		atIdx := 0
		for i, c := range email {
			if c == '@' {
				atIdx = i
				break
			}
		}
		if atIdx > 0 {
			name = email[:atIdx]
		} else {
			name = email
		}
	}

	user, exists := s.users[email]
	if !exists {
		// Auto-create user for mock OAuth sign-in
		user = &mockUser{
			Sub:       uuid.New().String(),
			Email:     email,
			Name:      name,
			Confirmed: true,
		}
		s.users[email] = user
	}

	// Generate tokens
	accessToken := generateToken()
	refreshToken := generateToken()
	mockIDToken := generateToken()

	user.AccessToken = accessToken
	user.RefreshToken = refreshToken
	user.IDToken = mockIDToken
	user.TokenExpiry = time.Now().Add(1 * time.Hour)

	s.tokens[accessToken] = user
	s.refreshTokens[refreshToken] = user
	s.save()

	fmt.Printf("[MOCK AUTH] OAuth sign-in (ValidateIDToken): %s\n", email)

	userInfo := &UserInfo{
		Sub:      user.Sub,
		Username: user.Email,
		Email:    user.Email,
		Name:     user.Name,
		Picture:  user.Picture,
	}

	tokens := &AuthTokens{
		AccessToken:  accessToken,
		IDToken:      mockIDToken,
		RefreshToken: refreshToken,
		ExpiresIn:    3600,
	}

	return userInfo, tokens, nil
}
