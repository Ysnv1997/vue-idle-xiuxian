package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/kowming/vue-idle-xiuxian/backend/internal/repository"
)

type AuthService struct {
	users         *repository.UserRepository
	tokens        *TokenService
	runtimeConfig *RuntimeConfigService
}

type LoginResult struct {
	User      *repository.User `json:"user"`
	TokenPair TokenPair        `json:"token"`
}

type RegistrationLimitReachedError struct {
	Limit   int
	Current int
}

func (e *RegistrationLimitReachedError) Error() string {
	return fmt.Sprintf("open registration limit reached: current=%d limit=%d", e.Current, e.Limit)
}

func NewAuthService(
	users *repository.UserRepository,
	tokens *TokenService,
	runtimeConfig *RuntimeConfigService,
) *AuthService {
	return &AuthService{
		users:         users,
		tokens:        tokens,
		runtimeConfig: runtimeConfig,
	}
}

func (s *AuthService) LoginByLinuxDoUser(ctx context.Context, linuxDoUserID, username, avatar string) (LoginResult, error) {
	registrationLimit := s.getOpenRegistrationLimit(ctx)
	user, err := s.users.UpsertLinuxDoUserWithRegistrationLimit(ctx, linuxDoUserID, username, avatar, registrationLimit)
	if err != nil {
		var limitErr *repository.UserRegistrationLimitReachedError
		if errors.As(err, &limitErr) {
			return LoginResult{}, &RegistrationLimitReachedError{
				Limit:   limitErr.Limit,
				Current: limitErr.Current,
			}
		}
		return LoginResult{}, fmt.Errorf("upsert linux do user: %w", err)
	}

	tokenPair, err := s.tokens.IssueTokenPair(user.ID, user.LinuxDoUserID)
	if err != nil {
		return LoginResult{}, fmt.Errorf("issue token pair: %w", err)
	}

	return LoginResult{User: user, TokenPair: tokenPair}, nil
}

func (s *AuthService) getOpenRegistrationLimit(ctx context.Context) int {
	if s == nil || s.runtimeConfig == nil {
		return 0
	}
	return s.runtimeConfig.GetInt(ctx, RuntimeConfigKeyAuthOpenRegistrationLimit, 0, 0, 100000000)
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
