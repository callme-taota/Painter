package application

import (
	"time"

	"painter-2026/identity-service/internal/domain"
	"painter-2026/identity-service/internal/infrastructure/token"
)

type AuthService struct {
	issuer token.Issuer
}

type LoginResult struct {
	UserID      string `json:"userId"`
	AccessToken string `json:"accessToken"`
	ExpiresAt   string `json:"expiresAt"`
}

func NewAuthService(issuer token.Issuer) *AuthService {
	return &AuthService{issuer: issuer}
}

func (s *AuthService) Login(cmd domain.LoginCommand) (LoginResult, error) {
	if err := cmd.Validate(); err != nil {
		return LoginResult{}, err
	}
	expireAt := time.Now().Add(2 * time.Hour)
	return LoginResult{
		UserID:      "u-admin",
		AccessToken: s.issuer.Issue(cmd.Username, expireAt),
		ExpiresAt:   expireAt.Format(time.RFC3339),
	}, nil
}
