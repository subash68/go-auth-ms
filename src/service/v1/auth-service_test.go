package v1

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	v1 "github.com/subash68/authenticator/src/api/v1"
)

// ─── Helpers ──────────────────────────────────────────────────────────────────

func newTestServer(t *testing.T) (v1.AuthServiceServer, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewAuthServiceServer(db), mock
}

func grpcCode(err error) codes.Code {
	if err == nil {
		return codes.OK
	}
	s, ok := status.FromError(err)
	if !ok {
		return codes.Unknown
	}
	return s.Code()
}

func mustHashPassword(t *testing.T, pw string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	return string(h)
}

// ─── Register ─────────────────────────────────────────────────────────────────

func TestRegister(t *testing.T) {
	const (
		validUser  = "alice"
		validPass  = "secret123"
		validFirst = "Alice"
		validLast  = "Smith"
	)

	validReq := &v1.RegisterRequest{
		Username:  validUser,
		Password:  validPass,
		FirstName: validFirst,
		LastName:  validLast,
	}

	tests := []struct {
		name     string
		req      *v1.RegisterRequest
		setup    func(sqlmock.Sqlmock)
		wantCode codes.Code
	}{
		{
			name: "success",
			req:  validReq,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectBegin()
				m.ExpectExec("INSERT INTO users").
					WithArgs(sqlmock.AnyArg(), validUser, validFirst, validLast, nil, sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectExec("INSERT INTO user_roles").
					WithArgs(sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectCommit()
			},
			wantCode: codes.OK,
		},
		{
			name: "success with optional email",
			req: &v1.RegisterRequest{
				Username: "bob", Password: validPass,
				FirstName: "Bob", LastName: "Jones", Email: "bob@example.com",
			},
			setup: func(m sqlmock.Sqlmock) {
				email := "bob@example.com"
				m.ExpectBegin()
				m.ExpectExec("INSERT INTO users").
					WithArgs(sqlmock.AnyArg(), "bob", "Bob", "Jones", &email, sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectExec("INSERT INTO user_roles").
					WithArgs(sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectCommit()
			},
			wantCode: codes.OK,
		},
		{
			name:     "missing username",
			req:      &v1.RegisterRequest{Password: validPass, FirstName: validFirst, LastName: validLast},
			setup:    func(m sqlmock.Sqlmock) {},
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "missing password",
			req:      &v1.RegisterRequest{Username: validUser, FirstName: validFirst, LastName: validLast},
			setup:    func(m sqlmock.Sqlmock) {},
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "missing first_name",
			req:      &v1.RegisterRequest{Username: validUser, Password: validPass, LastName: validLast},
			setup:    func(m sqlmock.Sqlmock) {},
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "missing last_name",
			req:      &v1.RegisterRequest{Username: validUser, Password: validPass, FirstName: validFirst},
			setup:    func(m sqlmock.Sqlmock) {},
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "invalid username with special chars",
			req:      &v1.RegisterRequest{Username: "alice!", Password: validPass, FirstName: validFirst, LastName: validLast},
			setup:    func(m sqlmock.Sqlmock) {},
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "invalid username with spaces",
			req:      &v1.RegisterRequest{Username: "alice bob", Password: validPass, FirstName: validFirst, LastName: validLast},
			setup:    func(m sqlmock.Sqlmock) {},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "duplicate username",
			req:  validReq,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectBegin()
				m.ExpectExec("INSERT INTO users").
					WillReturnError(fmt.Errorf("pq: duplicate key value violates unique constraint (23505)"))
				m.ExpectRollback()
			},
			wantCode: codes.AlreadyExists,
		},
		{
			name: "begin tx failure",
			req:  validReq,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectBegin().WillReturnError(fmt.Errorf("connection error"))
			},
			wantCode: codes.Internal,
		},
		{
			name: "insert user db error",
			req:  validReq,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectBegin()
				m.ExpectExec("INSERT INTO users").
					WillReturnError(fmt.Errorf("db error"))
				m.ExpectRollback()
			},
			wantCode: codes.Internal,
		},
		{
			name: "assign role db error",
			req:  validReq,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectBegin()
				m.ExpectExec("INSERT INTO users").
					WithArgs(sqlmock.AnyArg(), validUser, validFirst, validLast, nil, sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectExec("INSERT INTO user_roles").
					WillReturnError(fmt.Errorf("role table error"))
				m.ExpectRollback()
			},
			wantCode: codes.Internal,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, mock := newTestServer(t)
			tc.setup(mock)
			resp, err := srv.Register(context.Background(), tc.req)
			if got := grpcCode(err); got != tc.wantCode {
				t.Errorf("code: got %v, want %v (err: %v)", got, tc.wantCode, err)
			}
			if tc.wantCode == codes.OK {
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				if resp.UserId == "" {
					t.Error("expected non-empty user_id")
				}
				if resp.Username != tc.req.Username {
					t.Errorf("username: got %q, want %q", resp.Username, tc.req.Username)
				}
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unmet expectations: %v", err)
			}
		})
	}
}

// ─── Login ────────────────────────────────────────────────────────────────────

func TestLogin(t *testing.T) {
	const (
		userID    = "11111111-1111-1111-1111-111111111111"
		username  = "alice"
		password  = "secret123"
		firstName = "Alice"
		lastName  = "Smith"
	)

	// Pre-compute hash once to avoid repeated bcrypt work.
	hash := mustHashPassword(t, password)

	loginCols := []string{"id", "password_hash", "first_name", "last_name", "email", "is_active"}

	tests := []struct {
		name     string
		req      *v1.LoginRequest
		setup    func(sqlmock.Sqlmock)
		wantCode codes.Code
	}{
		{
			name: "success",
			req:  &v1.LoginRequest{Username: username, Password: password},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT id, password_hash").
					WithArgs(username).
					WillReturnRows(sqlmock.NewRows(loginCols).
						AddRow(userID, hash, firstName, lastName, sql.NullString{}, true))
				m.ExpectExec("INSERT INTO sessions").
					WithArgs(sqlmock.AnyArg(), userID, sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantCode: codes.OK,
		},
		{
			name: "success with email",
			req:  &v1.LoginRequest{Username: username, Password: password},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT id, password_hash").
					WithArgs(username).
					WillReturnRows(sqlmock.NewRows(loginCols).
						AddRow(userID, hash, firstName, lastName,
							sql.NullString{String: "alice@example.com", Valid: true}, true))
				m.ExpectExec("INSERT INTO sessions").
					WithArgs(sqlmock.AnyArg(), userID, sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantCode: codes.OK,
		},
		{
			name:     "missing username",
			req:      &v1.LoginRequest{Password: password},
			setup:    func(m sqlmock.Sqlmock) {},
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "missing password",
			req:      &v1.LoginRequest{Username: username},
			setup:    func(m sqlmock.Sqlmock) {},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "user not found",
			req:  &v1.LoginRequest{Username: "ghost", Password: password},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT id, password_hash").
					WithArgs("ghost").
					WillReturnRows(sqlmock.NewRows(loginCols))
			},
			wantCode: codes.Unauthenticated,
		},
		{
			name: "wrong password",
			req:  &v1.LoginRequest{Username: username, Password: "wrongpass"},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT id, password_hash").
					WithArgs(username).
					WillReturnRows(sqlmock.NewRows(loginCols).
						AddRow(userID, hash, firstName, lastName, sql.NullString{}, true))
			},
			wantCode: codes.Unauthenticated,
		},
		{
			name: "inactive account",
			req:  &v1.LoginRequest{Username: username, Password: password},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT id, password_hash").
					WithArgs(username).
					WillReturnRows(sqlmock.NewRows(loginCols).
						AddRow(userID, hash, firstName, lastName, sql.NullString{}, false))
			},
			wantCode: codes.PermissionDenied,
		},
		{
			name: "db query error",
			req:  &v1.LoginRequest{Username: username, Password: password},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT id, password_hash").
					WithArgs(username).
					WillReturnError(fmt.Errorf("connection lost"))
			},
			wantCode: codes.Internal,
		},
		{
			name: "session insert error",
			req:  &v1.LoginRequest{Username: username, Password: password},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT id, password_hash").
					WithArgs(username).
					WillReturnRows(sqlmock.NewRows(loginCols).
						AddRow(userID, hash, firstName, lastName, sql.NullString{}, true))
				m.ExpectExec("INSERT INTO sessions").
					WillReturnError(fmt.Errorf("insert failed"))
			},
			wantCode: codes.Internal,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, mock := newTestServer(t)
			tc.setup(mock)
			resp, err := srv.Login(context.Background(), tc.req)
			if got := grpcCode(err); got != tc.wantCode {
				t.Errorf("code: got %v, want %v (err: %v)", got, tc.wantCode, err)
			}
			if tc.wantCode == codes.OK {
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				if resp.AccessToken == "" {
					t.Error("expected non-empty access_token")
				}
				if resp.RefreshToken == "" {
					t.Error("expected non-empty refresh_token")
				}
				if resp.TokenType != "Bearer" {
					t.Errorf("token_type: got %q, want Bearer", resp.TokenType)
				}
				if resp.ExpiresIn != int64(accessTokenTTL.Seconds()) {
					t.Errorf("expires_in: got %d, want %d", resp.ExpiresIn, int64(accessTokenTTL.Seconds()))
				}
				if resp.User == nil {
					t.Fatal("expected non-nil user in response")
				}
				if resp.User.Id != userID {
					t.Errorf("user.id: got %q, want %q", resp.User.Id, userID)
				}
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unmet expectations: %v", err)
			}
		})
	}
}

// ─── Logout ───────────────────────────────────────────────────────────────────

func TestLogout(t *testing.T) {
	const token = "valid-refresh-token-abc123"

	tests := []struct {
		name     string
		req      *v1.LogoutRequest
		setup    func(sqlmock.Sqlmock)
		wantCode codes.Code
	}{
		{
			name: "success",
			req:  &v1.LogoutRequest{RefreshToken: token},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec("UPDATE sessions").
					WithArgs(token).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantCode: codes.OK,
		},
		{
			name:     "missing refresh_token",
			req:      &v1.LogoutRequest{},
			setup:    func(m sqlmock.Sqlmock) {},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "token not found or already revoked",
			req:  &v1.LogoutRequest{RefreshToken: token},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec("UPDATE sessions").
					WithArgs(token).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantCode: codes.NotFound,
		},
		{
			name: "db error",
			req:  &v1.LogoutRequest{RefreshToken: token},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec("UPDATE sessions").
					WithArgs(token).
					WillReturnError(fmt.Errorf("db connection lost"))
			},
			wantCode: codes.Internal,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, mock := newTestServer(t)
			tc.setup(mock)
			resp, err := srv.Logout(context.Background(), tc.req)
			if got := grpcCode(err); got != tc.wantCode {
				t.Errorf("code: got %v, want %v (err: %v)", got, tc.wantCode, err)
			}
			if tc.wantCode == codes.OK {
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				if !resp.Success {
					t.Error("expected success=true")
				}
				if resp.Message == "" {
					t.Error("expected non-empty message")
				}
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unmet expectations: %v", err)
			}
		})
	}
}

// ─── RefreshToken ─────────────────────────────────────────────────────────────

func TestRefreshToken(t *testing.T) {
	const (
		refreshTok = "valid-refresh-token-xyz789"
		userID     = "22222222-2222-2222-2222-222222222222"
		username   = "alice"
	)

	refreshCols := []string{"user_id", "username"}

	tests := []struct {
		name     string
		req      *v1.RefreshTokenRequest
		setup    func(sqlmock.Sqlmock)
		wantCode codes.Code
	}{
		{
			name: "success",
			req:  &v1.RefreshTokenRequest{RefreshToken: refreshTok},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT s.user_id").
					WithArgs(refreshTok).
					WillReturnRows(sqlmock.NewRows(refreshCols).AddRow(userID, username))
			},
			wantCode: codes.OK,
		},
		{
			name:     "missing refresh_token",
			req:      &v1.RefreshTokenRequest{},
			setup:    func(m sqlmock.Sqlmock) {},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "token invalid or expired",
			req:  &v1.RefreshTokenRequest{RefreshToken: "expired-token"},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT s.user_id").
					WithArgs("expired-token").
					WillReturnRows(sqlmock.NewRows(refreshCols))
			},
			wantCode: codes.Unauthenticated,
		},
		{
			name: "db error",
			req:  &v1.RefreshTokenRequest{RefreshToken: refreshTok},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT s.user_id").
					WithArgs(refreshTok).
					WillReturnError(fmt.Errorf("db connection lost"))
			},
			wantCode: codes.Internal,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, mock := newTestServer(t)
			tc.setup(mock)
			resp, err := srv.RefreshToken(context.Background(), tc.req)
			if got := grpcCode(err); got != tc.wantCode {
				t.Errorf("code: got %v, want %v (err: %v)", got, tc.wantCode, err)
			}
			if tc.wantCode == codes.OK {
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				if resp.AccessToken == "" {
					t.Error("expected non-empty access_token")
				}
				if resp.TokenType != "Bearer" {
					t.Errorf("token_type: got %q, want Bearer", resp.TokenType)
				}
				if resp.ExpiresIn != int64(accessTokenTTL.Seconds()) {
					t.Errorf("expires_in: got %d, want %d", resp.ExpiresIn, int64(accessTokenTTL.Seconds()))
				}
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unmet expectations: %v", err)
			}
		})
	}
}

// ─── Unimplemented stubs ──────────────────────────────────────────────────────

func TestGetProfile_Unimplemented(t *testing.T) {
	srv, _ := newTestServer(t)
	_, err := srv.GetProfile(context.Background(), &v1.GetProfileRequest{UserId: "any"})
	if got := grpcCode(err); got != codes.Unimplemented {
		t.Errorf("code: got %v, want Unimplemented", got)
	}
}

func TestAssignRole_Unimplemented(t *testing.T) {
	srv, _ := newTestServer(t)
	_, err := srv.AssignRole(context.Background(), &v1.AssignRoleRequest{UserId: "any", RoleName: "admin"})
	if got := grpcCode(err); got != codes.Unimplemented {
		t.Errorf("code: got %v, want Unimplemented", got)
	}
}

func TestGetPermissions_Unimplemented(t *testing.T) {
	srv, _ := newTestServer(t)
	_, err := srv.GetPermissions(context.Background(), &v1.GetPermissionsRequest{UserId: "any"})
	if got := grpcCode(err); got != codes.Unimplemented {
		t.Errorf("code: got %v, want Unimplemented", got)
	}
}
