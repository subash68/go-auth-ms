package v1

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"regexp"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	v1 "github.com/subash68/authenticator/src/api/v1"
)

const (
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 7 * 24 * time.Hour

	// JWTSecret should come from env/config in production.
	jwtSecret = "change-me-in-production"
)

var usernameRE = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

type authServiceServer struct {
	v1.UnimplementedAuthServiceServer
	db *sql.DB
}

func NewAuthServiceServer(db *sql.DB) v1.AuthServiceServer {
	return &authServiceServer{db: db}
}

// ─────────────────────────────────────────────────────────────
// Register
// ─────────────────────────────────────────────────────────────

func (s *authServiceServer) Register(ctx context.Context, req *v1.RegisterRequest) (*v1.RegisterResponse, error) {
	if req.Username == "" || req.Password == "" || req.FirstName == "" || req.LastName == "" {
		return nil, status.Error(codes.InvalidArgument, "username, password, first_name and last_name are required")
	}
	if !usernameRE.MatchString(req.Username) {
		return nil, status.Error(codes.InvalidArgument, "username must be alphanumeric (a-z, A-Z, 0-9, underscore)")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to hash password")
	}

	userID := uuid.New().String()
	var email *string
	if req.Email != "" {
		email = &req.Email
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to begin transaction")
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO users (id, username, first_name, last_name, email, password_hash)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		userID, req.Username, req.FirstName, req.LastName, email, string(hash),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, status.Error(codes.AlreadyExists, "username or email already taken")
		}
		return nil, status.Error(codes.Internal, "failed to create user: "+err.Error())
	}

	// Assign default 'user' role
	_, err = tx.ExecContext(ctx,
		`INSERT INTO user_roles (user_id, role_id)
		 SELECT $1, id FROM roles WHERE name = 'user'`,
		userID,
	)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to assign default role: "+err.Error())
	}

	if err := tx.Commit(); err != nil {
		return nil, status.Error(codes.Internal, "failed to commit transaction")
	}

	return &v1.RegisterResponse{
		UserId:   userID,
		Username: req.Username,
	}, nil
}

// ─────────────────────────────────────────────────────────────
// Login
// ─────────────────────────────────────────────────────────────

func (s *authServiceServer) Login(ctx context.Context, req *v1.LoginRequest) (*v1.LoginResponse, error) {
	if req.Username == "" || req.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "username and password are required")
	}

	var (
		userID       string
		passwordHash string
		firstName    string
		lastName     string
		email        sql.NullString
		isActive     bool
	)

	err := s.db.QueryRowContext(ctx,
		`SELECT id, password_hash, first_name, last_name, email, is_active
		 FROM users
		 WHERE username = $1 AND deleted_at IS NULL`,
		req.Username,
	).Scan(&userID, &passwordHash, &firstName, &lastName, &email, &isActive)

	if err == sql.ErrNoRows {
		return nil, status.Error(codes.Unauthenticated, "invalid username or password")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to query user: "+err.Error())
	}
	if !isActive {
		return nil, status.Error(codes.PermissionDenied, "account is inactive")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid username or password")
	}

	accessToken, err := generateAccessToken(userID, req.Username)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to generate access token")
	}

	refreshToken, err := generateRefreshToken()
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to generate refresh token")
	}

	expiresAt := time.Now().Add(refreshTokenTTL)
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO sessions (id, user_id, refresh_token, expires_at)
		 VALUES ($1, $2, $3, $4)`,
		uuid.New().String(), userID, refreshToken, expiresAt,
	)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to create session: "+err.Error())
	}

	emailStr := ""
	if email.Valid {
		emailStr = email.String
	}

	return &v1.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(accessTokenTTL.Seconds()),
		User: &v1.User{
			Id:        userID,
			Username:  req.Username,
			FirstName: firstName,
			LastName:  lastName,
			Email:     emailStr,
			IsActive:  isActive,
		},
	}, nil
}

// ─────────────────────────────────────────────────────────────
// Logout
// ─────────────────────────────────────────────────────────────

func (s *authServiceServer) Logout(ctx context.Context, req *v1.LogoutRequest) (*v1.LogoutResponse, error) {
	if req.RefreshToken == "" {
		return nil, status.Error(codes.InvalidArgument, "refresh_token is required")
	}

	res, err := s.db.ExecContext(ctx,
		`UPDATE sessions
		 SET is_revoked = TRUE, revoked_at = NOW()
		 WHERE refresh_token = $1 AND is_revoked = FALSE`,
		req.RefreshToken,
	)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to revoke session: "+err.Error())
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return nil, status.Error(codes.NotFound, "session not found or already revoked")
	}

	return &v1.LogoutResponse{
		Success: true,
		Message: "logged out successfully",
	}, nil
}

// ─────────────────────────────────────────────────────────────
// RefreshToken
// ─────────────────────────────────────────────────────────────

func (s *authServiceServer) RefreshToken(ctx context.Context, req *v1.RefreshTokenRequest) (*v1.RefreshTokenResponse, error) {
	if req.RefreshToken == "" {
		return nil, status.Error(codes.InvalidArgument, "refresh_token is required")
	}

	var (
		userID   string
		username string
	)

	err := s.db.QueryRowContext(ctx,
		`SELECT s.user_id, u.username
		 FROM sessions s
		 JOIN users u ON u.id = s.user_id
		 WHERE s.refresh_token = $1
		   AND s.is_revoked = FALSE
		   AND s.expires_at > NOW()
		   AND u.deleted_at IS NULL
		   AND u.is_active = TRUE`,
		req.RefreshToken,
	).Scan(&userID, &username)

	if err == sql.ErrNoRows {
		return nil, status.Error(codes.Unauthenticated, "refresh token is invalid or expired")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to validate refresh token: "+err.Error())
	}

	accessToken, err := generateAccessToken(userID, username)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to generate access token")
	}

	return &v1.RefreshTokenResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int64(accessTokenTTL.Seconds()),
	}, nil
}

// ─────────────────────────────────────────────────────────────
// GetProfile / AssignRole / GetPermissions — stubs (not in scope)
// ─────────────────────────────────────────────────────────────

func (s *authServiceServer) GetProfile(ctx context.Context, req *v1.GetProfileRequest) (*v1.GetProfileResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (s *authServiceServer) AssignRole(ctx context.Context, req *v1.AssignRoleRequest) (*v1.AssignRoleResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (s *authServiceServer) GetPermissions(ctx context.Context, req *v1.GetPermissionsRequest) (*v1.GetPermissionsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

// ─────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────

type jwtClaims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func generateAccessToken(userID, username string) (string, error) {
	claims := jwtClaims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(accessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   userID,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(jwtSecret))
}

func generateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// isUniqueViolation detects postgres unique constraint errors (code 23505).
func isUniqueViolation(err error) bool {
	return err != nil && (contains(err.Error(), "23505") || contains(err.Error(), "unique"))
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
