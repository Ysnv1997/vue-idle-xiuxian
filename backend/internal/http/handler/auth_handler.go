package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/kowming/vue-idle-xiuxian/backend/internal/config"
	"github.com/kowming/vue-idle-xiuxian/backend/internal/http/middleware"
	"github.com/kowming/vue-idle-xiuxian/backend/internal/repository"
	"github.com/kowming/vue-idle-xiuxian/backend/internal/service"
)

type AuthHandler struct {
	cfg          config.Config
	authService  *service.AuthService
	userRepo     *repository.UserRepository
	adminService *service.AdminService
	httpClient   *http.Client
}

func NewAuthHandler(
	cfg config.Config,
	authService *service.AuthService,
	userRepo *repository.UserRepository,
	adminService *service.AdminService,
) *AuthHandler {
	return &AuthHandler{
		cfg:          cfg,
		authService:  authService,
		userRepo:     userRepo,
		adminService: adminService,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type devLoginRequest struct {
	LinuxDoUserID string `json:"linuxDoUserId"`
	Username      string `json:"username"`
	Avatar        string `json:"avatar"`
}

func (h *AuthHandler) DevLogin(c *gin.Context) {
	if !h.cfg.EnableDevLogin {
		c.JSON(http.StatusNotFound, gin.H{"error": "dev login is disabled"})
		return
	}

	var req devLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload"})
		return
	}

	if strings.TrimSpace(req.LinuxDoUserID) == "" {
		req.LinuxDoUserID = "dev-" + uuid.NewString()
	}
	if strings.TrimSpace(req.Username) == "" {
		suffix := req.LinuxDoUserID
		if len(suffix) > 6 {
			suffix = suffix[len(suffix)-6:]
		}
		req.Username = "道友" + suffix
	}

	result, err := h.authService.LoginByLinuxDoUser(c.Request.Context(), req.LinuxDoUserID, req.Username, req.Avatar)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "login failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":            result.User.ID,
			"linuxDoUserId": result.User.LinuxDoUserID,
			"username":      result.User.LinuxDoUsername,
			"avatar":        result.User.LinuxDoAvatar,
		},
		"token": result.TokenPair,
	})
}

func (h *AuthHandler) LinuxDoAuthorize(c *gin.Context) {
	if h.cfg.LinuxDoClientID == "" || h.cfg.LinuxDoRedirectURL == "" {
		c.JSON(http.StatusNotImplemented, gin.H{
			"error":   "linux.do oauth is not configured",
			"message": "set LINUX_DO_CLIENT_ID and LINUX_DO_REDIRECT_URL to enable oauth",
		})
		return
	}

	redirectTarget := c.Query("redirect")
	if redirectTarget == "" {
		redirectTarget = h.cfg.FrontendLoginSuccessURL
	}

	stateToken, err := h.signOAuthState(redirectTarget)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create oauth state"})
		return
	}

	authorizeURL, err := url.Parse(h.cfg.LinuxDoAuthorizeURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid authorize url"})
		return
	}

	query := authorizeURL.Query()
	query.Set("response_type", "code")
	query.Set("client_id", h.cfg.LinuxDoClientID)
	query.Set("redirect_uri", h.cfg.LinuxDoRedirectURL)
	query.Set("scope", h.cfg.LinuxDoScope)
	query.Set("state", stateToken)
	authorizeURL.RawQuery = query.Encode()

	c.Redirect(http.StatusFound, authorizeURL.String())
}

func (h *AuthHandler) LinuxDoCallback(c *gin.Context) {
	if c.Query("error") != "" {
		h.redirectOAuthFailure(c, c.Query("state"), fmt.Sprintf("oauth_error:%s", c.Query("error")))
		return
	}

	code := c.Query("code")
	if strings.TrimSpace(code) == "" {
		h.redirectOAuthFailure(c, c.Query("state"), "missing_code")
		return
	}

	stateClaims, err := h.verifyOAuthState(c.Query("state"))
	if err != nil {
		h.redirectOAuthFailure(c, c.Query("state"), "invalid_state")
		return
	}

	accessToken, err := h.exchangeCodeForAccessToken(c.Request.Context(), code)
	if err != nil {
		log.Printf(
			"linux.do oauth token exchange failed: err=%v token_url=%s redirect_url=%s",
			err,
			h.cfg.LinuxDoTokenURL,
			h.cfg.LinuxDoRedirectURL,
		)
		h.redirectOAuthFailure(c, c.Query("state"), "token_exchange_failed")
		return
	}

	profile, err := h.fetchLinuxDoProfile(c.Request.Context(), accessToken)
	if err != nil {
		h.redirectOAuthFailure(c, c.Query("state"), "fetch_profile_failed")
		return
	}

	result, err := h.authService.LoginByLinuxDoUser(c.Request.Context(), profile.UserID, profile.Username, profile.Avatar)
	if err != nil {
		h.redirectOAuthFailure(c, c.Query("state"), "local_login_failed")
		return
	}

	redirectURL := buildRedirectURL(stateClaims.RedirectURL, url.Values{
		"accessToken":  {result.TokenPair.AccessToken},
		"refreshToken": {result.TokenPair.RefreshToken},
		"expiresIn":    {fmt.Sprintf("%d", result.TokenPair.ExpiresIn)},
		"login":        {"success"},
	})
	if redirectURL == "" {
		c.JSON(http.StatusOK, gin.H{
			"user": gin.H{
				"id":            result.User.ID,
				"linuxDoUserId": result.User.LinuxDoUserID,
				"username":      result.User.LinuxDoUsername,
				"avatar":        result.User.LinuxDoAvatar,
			},
			"token": result.TokenPair,
		})
		return
	}

	c.Redirect(http.StatusFound, redirectURL)
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.RefreshToken) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refreshToken is required"})
		return
	}

	tokens, err := h.authService.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "refresh failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": tokens})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	user, err := h.userRepo.FindByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query user failed"})
		return
	}
	if user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	adminProfile := service.AdminPermissionProfile{}
	if h.adminService != nil {
		profile, err := h.adminService.PermissionProfileByLinuxDoUserID(c.Request.Context(), user.LinuxDoUserID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "query admin status failed"})
			return
		}
		adminProfile = profile
	}

	c.JSON(http.StatusOK, gin.H{
		"id":                      user.ID,
		"linuxDoUserId":           user.LinuxDoUserID,
		"username":                user.LinuxDoUsername,
		"avatar":                  user.LinuxDoAvatar,
		"lastLoginAt":             user.LastLoginAt,
		"isAdmin":                 adminProfile.IsAdmin,
		"adminRole":               adminProfile.Role,
		"isSuperAdmin":            adminProfile.IsSuperAdmin,
		"canManageAdmins":         adminProfile.CanManageAdmins,
		"canManageRuntimeConfigs": adminProfile.CanManageRuntimeConfigs,
		"canModerateChat":         adminProfile.CanModerateChat,
	})
}

type oauthStateClaims struct {
	RedirectURL string `json:"redirectUrl"`
	jwt.RegisteredClaims
}

func (h *AuthHandler) signOAuthState(redirectURL string) (string, error) {
	now := time.Now().UTC()
	claims := oauthStateClaims{
		RedirectURL: redirectURL,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(h.cfg.OAuthStateTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        uuid.NewString(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(h.cfg.JWTSecret))
	if err != nil {
		return "", err
	}
	return signed, nil
}

func (h *AuthHandler) verifyOAuthState(stateToken string) (*oauthStateClaims, error) {
	if strings.TrimSpace(stateToken) == "" {
		return nil, fmt.Errorf("state token is empty")
	}

	parsed, err := jwt.ParseWithClaims(stateToken, &oauthStateClaims{}, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %s", t.Method.Alg())
		}
		return []byte(h.cfg.JWTSecret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired())
	if err != nil {
		return nil, err
	}

	claims, ok := parsed.Claims.(*oauthStateClaims)
	if !ok {
		return nil, fmt.Errorf("invalid oauth state claims")
	}
	if claims.RedirectURL == "" {
		claims.RedirectURL = h.cfg.FrontendLoginSuccessURL
	}
	return claims, nil
}

func (h *AuthHandler) exchangeCodeForAccessToken(ctx context.Context, code string) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", h.cfg.LinuxDoRedirectURL)
	form.Set("client_id", h.cfg.LinuxDoClientID)
	form.Set("client_secret", h.cfg.LinuxDoClientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.cfg.LinuxDoTokenURL, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("token endpoint status %d: %s", resp.StatusCode, string(body))
	}

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}

	accessToken, _ := payload["access_token"].(string)
	if strings.TrimSpace(accessToken) == "" {
		return "", fmt.Errorf("missing access_token in token response")
	}
	return accessToken, nil
}

type linuxDoProfile struct {
	UserID   string
	Username string
	Avatar   string
}

func (h *AuthHandler) fetchLinuxDoProfile(ctx context.Context, accessToken string) (*linuxDoProfile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.cfg.LinuxDoUserInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("userinfo endpoint status %d: %s", resp.StatusCode, string(body))
	}

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	profile := &linuxDoProfile{
		UserID:   pickString(payload, []string{"sub", "id", "user_id", "uid"}),
		Username: pickString(payload, []string{"username", "name", "login", "preferred_username"}),
		Avatar:   pickString(payload, []string{"avatar", "avatar_url", "picture"}),
	}
	if profile.UserID == "" {
		return nil, fmt.Errorf("missing unique user id in linux.do userinfo response")
	}
	if profile.Username == "" {
		profile.Username = "道友" + profile.UserID
	}

	return profile, nil
}

func pickString(payload map[string]any, keys []string) string {
	for _, key := range keys {
		value, ok := payload[key]
		if !ok || value == nil {
			continue
		}
		switch raw := value.(type) {
		case string:
			if strings.TrimSpace(raw) != "" {
				return raw
			}
		case float64:
			return fmt.Sprintf("%.0f", raw)
		case json.Number:
			return raw.String()
		}
	}
	return ""
}

func (h *AuthHandler) redirectOAuthFailure(c *gin.Context, state string, reason string) {
	redirectBase := h.cfg.FrontendLoginFailureURL
	if claims, err := h.verifyOAuthState(state); err == nil && claims.RedirectURL != "" {
		redirectBase = claims.RedirectURL
	}

	redirectURL := buildRedirectURL(redirectBase, url.Values{
		"error": {reason},
	})
	if redirectURL == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": reason})
		return
	}

	c.Redirect(http.StatusFound, redirectURL)
}

func buildRedirectURL(base string, params url.Values) string {
	if strings.TrimSpace(base) == "" {
		return ""
	}

	parsed, err := url.Parse(base)
	if err != nil {
		return ""
	}

	if parsed.Fragment != "" {
		fragment := parsed.Fragment
		if strings.Contains(fragment, "?") {
			fragment += "&" + params.Encode()
		} else {
			fragment += "?" + params.Encode()
		}
		parsed.Fragment = fragment
		return parsed.String()
	}

	query := parsed.Query()
	for key, values := range params {
		for _, value := range values {
			query.Add(key, value)
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
