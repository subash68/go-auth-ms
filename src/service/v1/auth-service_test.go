package v1

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	v1 "github.com/subash68/authenticator/src/api/v1"
)

// helper: create a server backed by a mock DB
func newTestServer(t *testing.T) (v1.AuthServiceServer, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewAuthServiceServer(db), mock
}

// helper: extract the gRPC code from an error
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

func TestCreate(t *testing.T) {
	tests := []struct {
		name     string
		req      *v1.CreateRequest
		setup    func(sqlmock.Sqlmock)
		wantID   int64
		wantCode codes.Code
	}{
		{
			name: "success",
			req:  &v1.CreateRequest{Api: "v1", Auth: &v1.Auth{Token: "tok1", Description: "desc1"}},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec("INSERT INTO Auth").WithArgs("tok1", "desc1").WillReturnResult(sqlmock.NewResult(42, 1))
			},
			wantID: 42, wantCode: codes.OK,
		},
		{
			name:     "wrong api version",
			req:      &v1.CreateRequest{Api: "v2", Auth: &v1.Auth{Token: "tok", Description: "desc"}},
			setup:    func(m sqlmock.Sqlmock) {},
			wantCode: codes.Unimplemented,
		},
		{
			name: "db insert error",
			req:  &v1.CreateRequest{Api: "v1", Auth: &v1.Auth{Token: "tok", Description: "desc"}},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec("INSERT INTO Auth").WithArgs("tok", "desc").WillReturnError(errors.New("insert failed"))
			},
			wantCode: codes.Unknown,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, mock := newTestServer(t)
			tc.setup(mock)
			resp, err := srv.Create(context.Background(), tc.req)
			if grpcCode(err) != tc.wantCode {
				t.Errorf("got code %v, want %v (err: %v)", grpcCode(err), tc.wantCode, err)
			}
			if tc.wantCode == codes.OK {
				if resp == nil { t.Fatal("expected non-nil response") }
				if resp.Id != tc.wantID { t.Errorf("got id %d, want %d", resp.Id, tc.wantID) }
				if resp.Api != apiVersion { t.Errorf("got api %q, want %q", resp.Api, apiVersion) }
			}
			if err := mock.ExpectationsWereMet(); err != nil { t.Errorf("unmet mock expectations: %v", err) }
		})
	}
}

func TestRead(t *testing.T) {
	cols := []string{"ID", "Token", "Description"}
	tests := []struct {
		name     string
		req      *v1.ReadRequest
		setup    func(sqlmock.Sqlmock)
		wantAuth *v1.Auth
		wantCode codes.Code
	}{
		{
			name: "success",
			req:  &v1.ReadRequest{Api: "v1", Id: 1},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT (.+) FROM Auth WHERE").WithArgs(int64(1)).
					WillReturnRows(sqlmock.NewRows(cols).AddRow(1, "tok1", "desc1"))
			},
			wantAuth: &v1.Auth{Id: 1, Token: "tok1", Description: "desc1"}, wantCode: codes.OK,
		},
		{
			name: "not found",
			req:  &v1.ReadRequest{Api: "v1", Id: 99},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT (.+) FROM Auth WHERE").WithArgs(int64(99)).WillReturnRows(sqlmock.NewRows(cols))
			},
			wantCode: codes.NotFound,
		},
		{
			name: "wrong api version", req: &v1.ReadRequest{Api: "v2", Id: 1},
			setup: func(m sqlmock.Sqlmock) {}, wantCode: codes.Unimplemented,
		},
		{
			name: "db query error",
			req:  &v1.ReadRequest{Api: "v1", Id: 1},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT (.+) FROM Auth WHERE").WithArgs(int64(1)).WillReturnError(errors.New("query failed"))
			},
			wantCode: codes.Unknown,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, mock := newTestServer(t)
			tc.setup(mock)
			resp, err := srv.Read(context.Background(), tc.req)
			if grpcCode(err) != tc.wantCode {
				t.Errorf("got code %v, want %v (err: %v)", grpcCode(err), tc.wantCode, err)
			}
			if tc.wantCode == codes.OK {
				if resp == nil || resp.Auth == nil { t.Fatal("expected non-nil response and auth") }
				if resp.Auth.Id != tc.wantAuth.Id || resp.Auth.Token != tc.wantAuth.Token || resp.Auth.Description != tc.wantAuth.Description {
					t.Errorf("got auth %+v, want %+v", resp.Auth, tc.wantAuth)
				}
			}
			if err := mock.ExpectationsWereMet(); err != nil { t.Errorf("unmet mock expectations: %v", err) }
		})
	}
}

func TestUpdate(t *testing.T) {
	tests := []struct {
		name        string
		req         *v1.UpdateRequest
		setup       func(sqlmock.Sqlmock)
		wantUpdated int64
		wantCode    codes.Code
	}{
		{
			name: "success",
			req:  &v1.UpdateRequest{Api: "v1", Auth: &v1.Auth{Id: 1, Token: "new-tok", Description: "new-desc"}},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec("UPDATE Auth").WithArgs("new-tok", "new-desc", int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantUpdated: 1, wantCode: codes.OK,
		},
		{
			name: "not found",
			req:  &v1.UpdateRequest{Api: "v1", Auth: &v1.Auth{Id: 99, Token: "tok", Description: "desc"}},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec("UPDATE Auth").WithArgs("tok", "desc", int64(99)).WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantCode: codes.NotFound,
		},
		{
			name: "wrong api version",
			req:  &v1.UpdateRequest{Api: "v2", Auth: &v1.Auth{Id: 1, Token: "tok", Description: "desc"}},
			setup: func(m sqlmock.Sqlmock) {}, wantCode: codes.Unimplemented,
		},
		{
			name: "db exec error",
			req:  &v1.UpdateRequest{Api: "v1", Auth: &v1.Auth{Id: 1, Token: "tok", Description: "desc"}},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec("UPDATE Auth").WithArgs("tok", "desc", int64(1)).WillReturnError(errors.New("update failed"))
			},
			wantCode: codes.Unknown,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, mock := newTestServer(t)
			tc.setup(mock)
			resp, err := srv.Update(context.Background(), tc.req)
			if grpcCode(err) != tc.wantCode {
				t.Errorf("got code %v, want %v (err: %v)", grpcCode(err), tc.wantCode, err)
			}
			if tc.wantCode == codes.OK {
				if resp == nil { t.Fatal("expected non-nil response") }
				if resp.Updated != tc.wantUpdated { t.Errorf("got updated %d, want %d", resp.Updated, tc.wantUpdated) }
			}
			if err := mock.ExpectationsWereMet(); err != nil { t.Errorf("unmet mock expectations: %v", err) }
		})
	}
}

func TestDelete(t *testing.T) {
	tests := []struct {
		name        string
		req         *v1.DeleteRequest
		setup       func(sqlmock.Sqlmock)
		wantDeleted int64
		wantCode    codes.Code
	}{
		{
			name: "success",
			req:  &v1.DeleteRequest{Api: "v1", Id: 1},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec("DELETE FROM Auth").WithArgs(int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantDeleted: 1, wantCode: codes.OK,
		},
		{
			name: "not found",
			req:  &v1.DeleteRequest{Api: "v1", Id: 99},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec("DELETE FROM Auth").WithArgs(int64(99)).WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantCode: codes.NotFound,
		},
		{
			name: "wrong api version", req: &v1.DeleteRequest{Api: "v2", Id: 1},
			setup: func(m sqlmock.Sqlmock) {}, wantCode: codes.Unimplemented,
		},
		{
			name: "db exec error",
			req:  &v1.DeleteRequest{Api: "v1", Id: 1},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec("DELETE FROM Auth").WithArgs(int64(1)).WillReturnError(errors.New("delete failed"))
			},
			wantCode: codes.Unknown,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, mock := newTestServer(t)
			tc.setup(mock)
			resp, err := srv.Delete(context.Background(), tc.req)
			if grpcCode(err) != tc.wantCode {
				t.Errorf("got code %v, want %v (err: %v)", grpcCode(err), tc.wantCode, err)
			}
			if tc.wantCode == codes.OK {
				if resp == nil { t.Fatal("expected non-nil response") }
				if resp.Deleted != tc.wantDeleted { t.Errorf("got deleted %d, want %d", resp.Deleted, tc.wantDeleted) }
			}
			if err := mock.ExpectationsWereMet(); err != nil { t.Errorf("unmet mock expectations: %v", err) }
		})
	}
}

func TestReadAll(t *testing.T) {
	cols := []string{"ID", "Token", "Description"}
	tests := []struct {
		name     string
		req      *v1.ReadAllRequest
		setup    func(sqlmock.Sqlmock)
		wantLen  int
		wantCode codes.Code
	}{
		{
			name: "success multiple rows",
			req:  &v1.ReadAllRequest{Api: "v1"},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT (.+) FROM Auth").
					WillReturnRows(sqlmock.NewRows(cols).AddRow(1, "tok1", "desc1").AddRow(2, "tok2", "desc2"))
			},
			wantLen: 2, wantCode: codes.OK,
		},
		{
			name: "empty table",
			req:  &v1.ReadAllRequest{Api: "v1"},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT (.+) FROM Auth").WillReturnRows(sqlmock.NewRows(cols))
			},
			wantLen: 0, wantCode: codes.OK,
		},
		{
			name: "wrong api version", req: &v1.ReadAllRequest{Api: "v2"},
			setup: func(m sqlmock.Sqlmock) {}, wantCode: codes.Unimplemented,
		},
		{
			name: "db query error",
			req:  &v1.ReadAllRequest{Api: "v1"},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT (.+) FROM Auth").WillReturnError(errors.New("query failed"))
			},
			wantCode: codes.Unknown,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, mock := newTestServer(t)
			tc.setup(mock)
			resp, err := srv.ReadAll(context.Background(), tc.req)
			if grpcCode(err) != tc.wantCode {
				t.Errorf("got code %v, want %v (err: %v)", grpcCode(err), tc.wantCode, err)
			}
			if tc.wantCode == codes.OK {
				if resp == nil { t.Fatal("expected non-nil response") }
				if len(resp.Auth) != tc.wantLen { t.Errorf("got %d rows, want %d", len(resp.Auth), tc.wantLen) }
				if resp.Api != apiVersion { t.Errorf("got api %q, want %q", resp.Api, apiVersion) }
			}
			if err := mock.ExpectationsWereMet(); err != nil { t.Errorf("unmet mock expectations: %v", err) }
		})
	}
}
