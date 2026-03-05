package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type TokenService struct {
	secret          []byte
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

type TokenPair struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int64  `json:"expiresIn"`
}

type Claims struct {
	UserID        string `json:"uid"`
	LinuxDoUserID string `json:"linuxDoUserId"`
	TokenType     string `json:"tokenType"`
	jwt.RegisteredClaims
}

func NewTokenService(secret string, accessTokenTTL, refreshTokenTTL time.Duration) *TokenService {
	return &TokenService{
		secret:          []byte(secret),
		accessTokenTTL:  accessTokenTTL,
		refreshTokenTTL: refreshTokenTTL,
	}
}

func (s *TokenService) IssueTokenPair(userID uuid.UUID, linuxDoUserID string) (TokenPair, error) {
	now := time.Now().UTC()
	accessToken, err := s.signToken(userID, linuxDoUserID, "access", now.Add(s.accessTokenTTL))
	if err != nil {
		return TokenPair{}, err
	}
	refreshToken, err := s.signToken(userID, linuxDoUserID, "refresh", now.Add(s.refreshTokenTTL))
	if err != nil {
		return TokenPair{}, err
	}

	return TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.accessTokenTTL.Seconds()),
	}, nil
}

func (s *TokenService) ValidateToken(token string, expectedTokenType string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %s", t.Method.Alg())
		}
		return s.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired())
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := parsed.Claims.(*Claims)
	if !ok {
		return nil, errors.New("invalid claims type")
	}
	if claims.TokenType != expectedTokenType {
		return nil, fmt.Errorf("invalid token type %q", claims.TokenType)
	}
	if claims.UserID == "" {
		return nil, errors.New("missing user id in token")
	}

	return claims, nil
}

func (s *TokenService) signToken(userID uuid.UUID, linuxDoUserID string, tokenType string, expiresAt time.Time) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		UserID:        userID.String(),
		LinuxDoUserID: linuxDoUserID,
		TokenType:     tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Subject:   userID.String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}
