package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/kowming/vue-idle-xiuxian/backend/internal/repository"
)

type AuthService struct {
	users  *repository.UserRepository
	tokens *TokenService
}

type LoginResult struct {
	User      *repository.User `json:"user"`
	TokenPair TokenPair        `json:"token"`
}

func NewAuthService(users *repository.UserRepository, tokens *TokenService) *AuthService {
	return &AuthService{users: users, tokens: tokens}
}

func (s *AuthService) LoginByLinuxDoUser(ctx context.Context, linuxDoUserID, username, avatar string) (LoginResult, error) {
	user, err := s.users.UpsertLinuxDoUser(ctx, linuxDoUserID, username, avatar)
	if err != nil {
		return LoginResult{}, fmt.Errorf("upsert linux do user: %w", err)
	}

	tokenPair, err := s.tokens.IssueTokenPair(user.ID, user.LinuxDoUserID)
	if err != nil {
		return LoginResult{}, fmt.Errorf("issue token pair: %w", err)
	}

	return LoginResult{User: user, TokenPair: tokenPair}, nil
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	claims, err := s.tokens.ValidateToken(refreshToken, "refresh")
	if err != nil {
		return TokenPair{}, fmt.Errorf("validate refresh token: %w", err)
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return TokenPair{}, fmt.Errorf("parse user id: %w", err)
	}

	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return TokenPair{}, fmt.Errorf("find user by id: %w", err)
	}
	if user == nil {
		return TokenPair{}, fmt.Errorf("user not found")
	}

	return s.tokens.IssueTokenPair(user.ID, user.LinuxDoUserID)
}
