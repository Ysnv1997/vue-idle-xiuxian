package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AdminRole string

const (
	AdminRoleSuperAdmin AdminRole = "super_admin"
	AdminRoleOpsAdmin   AdminRole = "ops_admin"
	AdminRoleChatAdmin  AdminRole = "chat_admin"
)

type AdminPermission string

const (
	AdminPermissionManageAdmins         AdminPermission = "manage_admins"
	AdminPermissionManageRuntimeConfigs AdminPermission = "manage_runtime_configs"
	AdminPermissionModerateChat         AdminPermission = "moderate_chat"
)

type AdminService struct {
	pool *pgxpool.Pool
}

type AdminUser struct {
	ID              int64      `json:"id"`
	LinuxDoUserID   string     `json:"linuxDoUserId"`
	Role            string     `json:"role"`
	Note            string     `json:"note"`
	Source          string     `json:"source"`
	CreatedByUserID *uuid.UUID `json:"createdByUserId,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

type AdminUserListResult struct {
	Users []AdminUser `json:"users"`
}

type AdminUserUpsertInput struct {
	LinuxDoUserID   string
	Role            string
	Note            string
	Source          string
	CreatedByUserID *uuid.UUID
}

type AdminPermissionProfile struct {
	IsAdmin                 bool   `json:"isAdmin"`
	Role                    string `json:"role"`
	IsSuperAdmin            bool   `json:"isSuperAdmin"`
	CanManageAdmins         bool   `json:"canManageAdmins"`
	CanManageRuntimeConfigs bool   `json:"canManageRuntimeConfigs"`
	CanModerateChat         bool   `json:"canModerateChat"`
}

func (p AdminPermissionProfile) Has(permission AdminPermission) bool {
	switch permission {
	case AdminPermissionManageAdmins:
		return p.CanManageAdmins
	case AdminPermissionManageRuntimeConfigs:
		return p.CanManageRuntimeConfigs
	case AdminPermissionModerateChat:
		return p.CanModerateChat
	default:
		return false
	}
}

type InvalidAdminLinuxDoUserIDError struct{}

func (e *InvalidAdminLinuxDoUserIDError) Error() string {
	return "invalid admin linux do user id"
}

type InvalidAdminRoleError struct {
	Role string
}

func (e *InvalidAdminRoleError) Error() string {
	return fmt.Sprintf("invalid admin role: %s", e.Role)
}

type AdminUserNotFoundError struct {
	LinuxDoUserID string
}

func (e *AdminUserNotFoundError) Error() string {
	return fmt.Sprintf("admin user not found: %s", e.LinuxDoUserID)
}

type AdminLastUserRemoveError struct {
	LinuxDoUserID string
}

func (e *AdminLastUserRemoveError) Error() string {
	return fmt.Sprintf("cannot remove last admin: %s", e.LinuxDoUserID)
}

type AdminLastSuperUserRemoveError struct {
	LinuxDoUserID string
}

func (e *AdminLastSuperUserRemoveError) Error() string {
	return fmt.Sprintf("cannot remove last super admin: %s", e.LinuxDoUserID)
}

type AdminLastSuperUserDemoteError struct {
	LinuxDoUserID string
}

func (e *AdminLastSuperUserDemoteError) Error() string {
	return fmt.Sprintf("cannot demote last super admin: %s", e.LinuxDoUserID)
}

type adminLockStats struct {
	TotalCount       int64
	SuperAdminCount  int64
	TargetExists     bool
	TargetRole       AdminRole
	TargetLinuxDoUID string
}

func NewAdminService(pool *pgxpool.Pool) *AdminService {
	return &AdminService{pool: pool}
}

func (s *AdminService) EnsureBootstrapAdmins(ctx context.Context, linuxDoUserIDs []string) error {
	if len(linuxDoUserIDs) == 0 {
		return nil
	}

	const query = `
		INSERT INTO game_admin_users (
			linux_do_user_id,
			role,
			note,
			source,
			created_by_user_id,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, 'bootstrap_env', NULL, now(), now())
		ON CONFLICT (linux_do_user_id) DO NOTHING
	`

	for _, raw := range linuxDoUserIDs {
		linuxDoUserID := normalizeLinuxDoUserID(raw)
		if linuxDoUserID == "" {
			continue
		}
		if _, err := s.pool.Exec(
			ctx,
			query,
			linuxDoUserID,
			string(AdminRoleSuperAdmin),
			"bootstrap from CHAT_ADMIN_USER_IDS",
		); err != nil {
			return fmt.Errorf("insert bootstrap admin %s: %w", linuxDoUserID, err)
		}
	}
	return nil
}

func (s *AdminService) PermissionProfileByLinuxDoUserID(ctx context.Context, linuxDoUserID string) (AdminPermissionProfile, error) {
	role, exists, err := s.RoleByLinuxDoUserID(ctx, linuxDoUserID)
	if err != nil {
		return AdminPermissionProfile{}, err
	}
	if !exists {
		return AdminPermissionProfile{}, nil
	}

	return AdminPermissionProfile{
		IsAdmin:                 true,
		Role:                    string(role),
		IsSuperAdmin:            role == AdminRoleSuperAdmin,
		CanManageAdmins:         RoleHasPermission(role, AdminPermissionManageAdmins),
		CanManageRuntimeConfigs: RoleHasPermission(role, AdminPermissionManageRuntimeConfigs),
		CanModerateChat:         RoleHasPermission(role, AdminPermissionModerateChat),
	}, nil
}

func (s *AdminService) HasPermissionByLinuxDoUserID(ctx context.Context, linuxDoUserID string, permission AdminPermission) (bool, error) {
	role, exists, err := s.RoleByLinuxDoUserID(ctx, linuxDoUserID)
	if err != nil || !exists {
		return false, err
	}
	return RoleHasPermission(role, permission), nil
}

func (s *AdminService) RoleByLinuxDoUserID(ctx context.Context, linuxDoUserID string) (AdminRole, bool, error) {
	linuxDoUserID = normalizeLinuxDoUserID(linuxDoUserID)
	if linuxDoUserID == "" {
		return "", false, nil
	}

	const query = `
		SELECT role
		FROM game_admin_users
		WHERE linux_do_user_id = $1
		LIMIT 1
	`
	var rawRole string
	if err := s.pool.QueryRow(ctx, query, linuxDoUserID).Scan(&rawRole); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("query admin role: %w", err)
	}

	role, ok := parseAdminRole(rawRole)
	if !ok {
		return "", false, fmt.Errorf("query admin role: invalid role in db: %s", rawRole)
	}
	return role, true, nil
}

func (s *AdminService) IsAdminByLinuxDoUserID(ctx context.Context, linuxDoUserID string) (bool, error) {
	_, exists, err := s.RoleByLinuxDoUserID(ctx, linuxDoUserID)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (s *AdminService) ListAdmins(ctx context.Context, limit int) (*AdminUserListResult, error) {
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}

	const query = `
		SELECT
			id,
			linux_do_user_id,
			role,
			COALESCE(note, ''),
			COALESCE(source, 'manual'),
			created_by_user_id,
			created_at,
			updated_at
		FROM game_admin_users
		ORDER BY created_at ASC, id ASC
		LIMIT $1
	`
	rows, err := s.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("query admin users: %w", err)
	}
	defer rows.Close()

	users := make([]AdminUser, 0, limit)
	for rows.Next() {
		user := AdminUser{}
		if err := rows.Scan(
			&user.ID,
			&user.LinuxDoUserID,
			&user.Role,
			&user.Note,
			&user.Source,
			&user.CreatedByUserID,
			&user.CreatedAt,
			&user.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan admin user: %w", err)
		}
		users = append(users, user)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate admin users: %w", rows.Err())
	}

	return &AdminUserListResult{Users: users}, nil
}

func (s *AdminService) UpsertAdmin(ctx context.Context, input AdminUserUpsertInput) (*AdminUser, error) {
	linuxDoUserID := normalizeLinuxDoUserID(input.LinuxDoUserID)
	if linuxDoUserID == "" {
		return nil, &InvalidAdminLinuxDoUserIDError{}
	}

	role, err := normalizeAdminRole(input.Role)
	if err != nil {
		return nil, err
	}
	note := strings.TrimSpace(input.Note)
	source := strings.TrimSpace(input.Source)
	if source == "" {
		source = "manual"
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin upsert admin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	currentRole, currentExists, err := s.lockTargetRole(ctx, tx, linuxDoUserID)
	if err != nil {
		return nil, err
	}
	if currentExists && currentRole == AdminRoleSuperAdmin && role != AdminRoleSuperAdmin {
		stats, statsErr := lockAndCountAdmins(ctx, tx, linuxDoUserID)
		if statsErr != nil {
			return nil, statsErr
		}
		if stats.SuperAdminCount <= 1 {
			return nil, &AdminLastSuperUserDemoteError{LinuxDoUserID: linuxDoUserID}
		}
	}

	const query = `
		INSERT INTO game_admin_users (
			linux_do_user_id,
			role,
			note,
			source,
			created_by_user_id,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, now(), now())
		ON CONFLICT (linux_do_user_id)
		DO UPDATE SET
			role = EXCLUDED.role,
			note = EXCLUDED.note,
			source = EXCLUDED.source,
			updated_at = now()
		RETURNING id, linux_do_user_id, role, COALESCE(note, ''), COALESCE(source, 'manual'), created_by_user_id, created_at, updated_at
	`

	adminUser := AdminUser{}
	if err := tx.QueryRow(ctx, query, linuxDoUserID, string(role), note, source, input.CreatedByUserID).Scan(
		&adminUser.ID,
		&adminUser.LinuxDoUserID,
		&adminUser.Role,
		&adminUser.Note,
		&adminUser.Source,
		&adminUser.CreatedByUserID,
		&adminUser.CreatedAt,
		&adminUser.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("upsert admin user: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit upsert admin transaction: %w", err)
	}
	return &adminUser, nil
}

func (s *AdminService) RemoveAdmin(ctx context.Context, linuxDoUserID string) (bool, error) {
	linuxDoUserID = normalizeLinuxDoUserID(linuxDoUserID)
	if linuxDoUserID == "" {
		return false, &InvalidAdminLinuxDoUserIDError{}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin remove admin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	stats, err := lockAndCountAdmins(ctx, tx, linuxDoUserID)
	if err != nil {
		return false, err
	}
	if !stats.TargetExists {
		return false, &AdminUserNotFoundError{LinuxDoUserID: linuxDoUserID}
	}
	if stats.TotalCount <= 1 {
		return false, &AdminLastUserRemoveError{LinuxDoUserID: linuxDoUserID}
	}
	if stats.TargetRole == AdminRoleSuperAdmin && stats.SuperAdminCount <= 1 {
		return false, &AdminLastSuperUserRemoveError{LinuxDoUserID: linuxDoUserID}
	}

	const deleteSQL = `
		DELETE FROM game_admin_users
		WHERE linux_do_user_id = $1
	`
	tag, err := tx.Exec(ctx, deleteSQL, linuxDoUserID)
	if err != nil {
		return false, fmt.Errorf("delete admin user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return false, &AdminUserNotFoundError{LinuxDoUserID: linuxDoUserID}
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit remove admin transaction: %w", err)
	}
	return true, nil
}

func (s *AdminService) lockTargetRole(ctx context.Context, tx pgx.Tx, linuxDoUserID string) (AdminRole, bool, error) {
	const query = `
		SELECT role
		FROM game_admin_users
		WHERE linux_do_user_id = $1
		FOR UPDATE
	`
	var rawRole string
	if err := tx.QueryRow(ctx, query, linuxDoUserID).Scan(&rawRole); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("load admin role for update: %w", err)
	}
	role, ok := parseAdminRole(rawRole)
	if !ok {
		return "", false, fmt.Errorf("load admin role for update: invalid role in db: %s", rawRole)
	}
	return role, true, nil
}

func lockAndCountAdmins(ctx context.Context, tx pgx.Tx, linuxDoUserID string) (adminLockStats, error) {
	const query = `
		SELECT linux_do_user_id, role
		FROM game_admin_users
		FOR UPDATE
	`
	rows, err := tx.Query(ctx, query)
	if err != nil {
		return adminLockStats{}, fmt.Errorf("lock admin users: %w", err)
	}
	defer rows.Close()

	stats := adminLockStats{TargetLinuxDoUID: linuxDoUserID}
	for rows.Next() {
		var currentLinuxDoUserID string
		var currentRoleRaw string
		if err := rows.Scan(&currentLinuxDoUserID, &currentRoleRaw); err != nil {
			return adminLockStats{}, fmt.Errorf("scan locked admin user: %w", err)
		}
		role, ok := parseAdminRole(currentRoleRaw)
		if !ok {
			return adminLockStats{}, fmt.Errorf("scan locked admin user: invalid role in db: %s", currentRoleRaw)
		}

		stats.TotalCount++
		if role == AdminRoleSuperAdmin {
			stats.SuperAdminCount++
		}
		if normalizeLinuxDoUserID(currentLinuxDoUserID) == linuxDoUserID {
			stats.TargetExists = true
			stats.TargetRole = role
		}
	}
	if rows.Err() != nil {
		return adminLockStats{}, fmt.Errorf("iterate locked admin users: %w", rows.Err())
	}
	return stats, nil
}

func normalizeLinuxDoUserID(value string) string {
	return strings.TrimSpace(value)
}

func normalizeAdminRole(value string) (AdminRole, error) {
	normalized := strings.TrimSpace(strings.ToLower(value))
	if normalized == "" {
		return AdminRoleSuperAdmin, nil
	}
	role, ok := parseAdminRole(normalized)
	if !ok {
		return "", &InvalidAdminRoleError{Role: value}
	}
	return role, nil
}

func parseAdminRole(value string) (AdminRole, bool) {
	switch AdminRole(strings.TrimSpace(strings.ToLower(value))) {
	case AdminRoleSuperAdmin:
		return AdminRoleSuperAdmin, true
	case AdminRoleOpsAdmin:
		return AdminRoleOpsAdmin, true
	case AdminRoleChatAdmin:
		return AdminRoleChatAdmin, true
	default:
		return "", false
	}
}

func RoleHasPermission(role AdminRole, permission AdminPermission) bool {
	switch role {
	case AdminRoleSuperAdmin:
		return true
	case AdminRoleOpsAdmin:
		return permission == AdminPermissionManageRuntimeConfigs
	case AdminRoleChatAdmin:
		return permission == AdminPermissionModerateChat
	default:
		return false
	}
}

func IsAdminServiceError(err error) bool {
	if err == nil {
		return false
	}

	var invalidErr *InvalidAdminLinuxDoUserIDError
	if errors.As(err, &invalidErr) {
		return true
	}
	var invalidRoleErr *InvalidAdminRoleError
	if errors.As(err, &invalidRoleErr) {
		return true
	}
	var notFoundErr *AdminUserNotFoundError
	if errors.As(err, &notFoundErr) {
		return true
	}
	var lastErr *AdminLastUserRemoveError
	if errors.As(err, &lastErr) {
		return true
	}
	var lastSuperRemoveErr *AdminLastSuperUserRemoveError
	if errors.As(err, &lastSuperRemoveErr) {
		return true
	}
	var lastSuperDemoteErr *AdminLastSuperUserDemoteError
	return errors.As(err, &lastSuperDemoteErr)
}
