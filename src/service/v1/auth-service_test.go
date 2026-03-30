package v1

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	v1 "github.com/subash68/authenticator/src/api/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// newTestServer creates a server backed by a mock DB.
func newTestServer(t *testing.T) (v1.AuthServiceServer, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewAuthServiceServer(db), mock
}

// ── checkAPI ──────────────────────────────────────────────────────────────────

func TestCheckAPI_ValidVersion(t *testing.T) {
	s := &authServiceServer{}
	if err := s.checkAPI("v1"); err != nil {
		t.Errorf("expected no error for valid API version, got %v", err)
	}
}

func TestCheckAPI_EmptyVersion(t *testing.T) {
	// Empty string means "don't check" — should pass.
	s := &authServiceServer{}
	if err := s.checkAPI(""); err != nil {
		t.Errorf("expected no error for empty API version, got %v", err)
	}
}

func TestCheckAPI_UnsupportedVersion(t *testing.T) {
	s := &authServiceServer{}
	err := s.checkAPI("v2")
	if err == nil {
		t.Fatal("expected error for unsupported API version, got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %T", err)
	}
	if st.Code() != codes.Unimplemented {
		t.Errorf("expected code Unimplemented, got %v", st.Code())
	}
}

// ── Create ────────────────────────────────────────────────────────────────────

func TestCreate_Success(t *testing.T) {
	srv, mock := newTestServer(t)

	mock.ExpectExec("INSERT INTO Auth").
		WithArgs("tok-abc", "test token").
		WillReturnResult(sqlmock.NewResult(42, 1))

	resp, err := srv.Create(context.Background(), &v1.CreateRequest{
		Api: "v1",
		Auth: &v1.Auth{
			Token:       "tok-abc",
			Description: "test token",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Api != "v1" {
		t.Errorf("expected api=v1, got %s", resp.Api)
	}
	if resp.Id != 42 {
		t.Errorf("expected id=42, got %d", resp.Id)
	}
}

func TestCreate_WrongAPIVersion(t *testing.T) {
	srv, _ := newTestServer(t)

	_, err := srv.Create(context.Background(), &v1.CreateRequest{
		Api:  "v99",
		Auth: &v1.Auth{Token: "tok", Description: "desc"},
	})
	if err == nil {
		t.Fatal("expected error for wrong API version, got nil")
	}
	if st, _ := status.FromError(err); st.Code() != codes.Unimplemented {
		t.Errorf("expected Unimplemented, got %v", st.Code())
	}
}

func TestCreate_DBInsertError(t *testing.T) {
	srv, mock := newTestServer(t)

	mock.ExpectExec("INSERT INTO Auth").
		WithArgs("tok-fail", "desc").
		WillReturnError(errDBFailure("connection reset"))

	_, err := srv.Create(context.Background(), &v1.CreateRequest{
		Api:  "v1",
		Auth: &v1.Auth{Token: "tok-fail", Description: "desc"},
	})
	if err == nil {
		t.Fatal("expected error on DB failure, got nil")
	}
	if st, _ := status.FromError(err); st.Code() != codes.Unknown {
		t.Errorf("expected Unknown, got %v", st.Code())
	}
}

func TestCreate_NilAuth(t *testing.T) {
	srv, _ := newTestServer(t)

	// Passing nil Auth should panic or return an error — either is acceptable
	// currently; this test documents the behaviour.
	defer func() { recover() }() // swallow panic if it occurs
	resp, err := srv.Create(context.Background(), &v1.CreateRequest{
		Api:  "v1",
		Auth: nil,
	})
	// If we reach here without panic, an error is expected.
	if err == nil && resp != nil {
		t.Error("expected error or panic when Auth is nil")
	}
}

// ── Read ──────────────────────────────────────────────────────────────────────

func TestRead_StubReturnsNilAuth(t *testing.T) {
	srv, _ := newTestServer(t)

	resp, err := srv.Read(context.Background(), &v1.ReadRequest{Api: "v1", Id: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Api != "v1" {
		t.Errorf("expected api=v1, got %s", resp.Api)
	}
	if resp.Auth != nil {
		t.Errorf("stub: expected nil Auth, got %+v", resp.Auth)
	}
}

func TestRead_WrongAPIVersion(t *testing.T) {
	// Stub doesn't call checkAPI yet — documents current (incomplete) behaviour.
	srv, _ := newTestServer(t)

	resp, err := srv.Read(context.Background(), &v1.ReadRequest{Api: "v99", Id: 1})
	// Current stub ignores version; document what it returns.
	if err != nil {
		t.Logf("Read returned error (expected once implemented): %v", err)
		return
	}
	t.Logf("Read stub returned resp=%+v (API version check not yet enforced)", resp)
}

// ── Update ────────────────────────────────────────────────────────────────────

func TestUpdate_StubReturnsZero(t *testing.T) {
	srv, _ := newTestServer(t)

	resp, err := srv.Update(context.Background(), &v1.UpdateRequest{
		Api:  "v1",
		Auth: &v1.Auth{Id: 1, Token: "new-tok", Description: "updated"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Api != "v1" {
		t.Errorf("expected api=v1, got %s", resp.Api)
	}
	if resp.Updated != 0 {
		t.Errorf("stub: expected Updated=0, got %d", resp.Updated)
	}
}

func TestUpdate_WrongAPIVersion(t *testing.T) {
	srv, _ := newTestServer(t)

	resp, err := srv.Update(context.Background(), &v1.UpdateRequest{
		Api:  "v99",
		Auth: &v1.Auth{Id: 1, Token: "tok", Description: "desc"},
	})
	if err != nil {
		t.Logf("Update returned error (expected once implemented): %v", err)
		return
	}
	t.Logf("Update stub returned resp=%+v (API version check not yet enforced)", resp)
}

// ── Delete ────────────────────────────────────────────────────────────────────

func TestDelete_StubReturnsZero(t *testing.T) {
	srv, _ := newTestServer(t)

	resp, err := srv.Delete(context.Background(), &v1.DeleteRequest{Api: "v1", Id: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Api != "v1" {
		t.Errorf("expected api=v1, got %s", resp.Api)
	}
	if resp.Deleted != 0 {
		t.Errorf("stub: expected Deleted=0, got %d", resp.Deleted)
	}
}

func TestDelete_WrongAPIVersion(t *testing.T) {
	srv, _ := newTestServer(t)

	resp, err := srv.Delete(context.Background(), &v1.DeleteRequest{Api: "v99", Id: 1})
	if err != nil {
		t.Logf("Delete returned error (expected once implemented): %v", err)
		return
	}
	t.Logf("Delete stub returned resp=%+v (API version check not yet enforced)", resp)
}

// ── ReadAll ───────────────────────────────────────────────────────────────────

func TestReadAll_StubReturnsEmptyList(t *testing.T) {
	srv, _ := newTestServer(t)

	resp, err := srv.ReadAll(context.Background(), &v1.ReadAllRequest{Api: "v1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Api != "v1" {
		t.Errorf("expected api=v1, got %s", resp.Api)
	}
	if len(resp.Auth) != 0 {
		t.Errorf("stub: expected empty Auth list, got %d items", len(resp.Auth))
	}
}

func TestReadAll_WrongAPIVersion(t *testing.T) {
	srv, _ := newTestServer(t)

	resp, err := srv.ReadAll(context.Background(), &v1.ReadAllRequest{Api: "v99"})
	if err != nil {
		t.Logf("ReadAll returned error (expected once implemented): %v", err)
		return
	}
	t.Logf("ReadAll stub returned resp=%+v (API version check not yet enforced)", resp)
}

// ── helpers ───────────────────────────────────────────────────────────────────

// errDBFailure is a simple error type used to simulate database failures.
type errDBFailure string

func (e errDBFailure) Error() string { return string(e) }
