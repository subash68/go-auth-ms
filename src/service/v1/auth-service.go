package v1

import (
	"context"
	"database/sql"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	v1 "github.com/subash68/authenticator/src/api/v1"
)

const (
	apiVersion = "v1"
)

type authServiceServer struct {
	db *sql.DB
}

// This dependency injection should be verified
func NewAuthServiceServer(db *sql.DB) v1.AuthServiceServer {
	return &authServiceServer{db: db}
}

func (s *authServiceServer) checkAPI(api string) error {
	if len(api) > 0 {
		if apiVersion != api {
			return status.Errorf(codes.Unimplemented, "unsupported API version: service implements API version '%s', but asked for '%s'", apiVersion, api)
		}
	}

	return nil
}

func (s *authServiceServer) connect(ctx context.Context) (*sql.Conn, error) {
	c, err := s.db.Conn(ctx)
	if err != nil {
		return nil, status.Error(codes.Unknown, "failed to connect to database-> "+err.Error())
	}
	return c, nil
}

// Create new todo task
func (s *authServiceServer) Create(ctx context.Context, req *v1.CreateRequest) (*v1.CreateResponse, error) {
	// check if the API version requested by client is supported by server
	if err := s.checkAPI(req.Api); err != nil {
		return nil, err
	}

	// get SQL connection from pool
	c, err := s.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	// reminder, err := ptypes.Timestamp(req.ToDo.Reminder)
	// if err != nil {
	// 	return nil, status.Error(codes.InvalidArgument, "reminder field has invalid format-> "+err.Error())
	// }

	// insert ToDo entity data
	res, err := c.ExecContext(ctx, "INSERT INTO Auth(`Token`, `Description`) VALUES(?, ?)",
		req.Auth.Token, req.Auth.Description)
	if err != nil {
		return nil, status.Error(codes.Unknown, "failed to insert into ToDo-> "+err.Error())
	}

	// get ID of creates ToDo
	id, err := res.LastInsertId()
	if err != nil {
		return nil, status.Error(codes.Unknown, "failed to retrieve id for created ToDo-> "+err.Error())
	}

	return &v1.CreateResponse{
		Api: apiVersion,
		Id:  id,
	}, nil
}

// Read todo task
func (s *authServiceServer) Read(ctx context.Context, req *v1.ReadRequest) (*v1.ReadResponse, error) {
	if err := s.checkAPI(req.Api); err != nil {
		return nil, err
	}

	c, err := s.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	rows, err := c.QueryContext(ctx, "SELECT `ID`, `Token`, `Description` FROM Auth WHERE `ID`=?", req.Id)
	if err != nil {
		return nil, status.Error(codes.Unknown, "failed to select from Auth-> "+err.Error())
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, status.Error(codes.Unknown, "failed to retrieve data from Auth-> "+err.Error())
		}
		return nil, status.Errorf(codes.NotFound, "Auth with ID='%d' is not found", req.Id)
	}

	var auth v1.Auth
	if err := rows.Scan(&auth.Id, &auth.Token, &auth.Description); err != nil {
		return nil, status.Error(codes.Unknown, "failed to retrieve field values from Auth row-> "+err.Error())
	}

	return &v1.ReadResponse{
		Api:  apiVersion,
		Auth: &auth,
	}, nil
}

// Update todo task
func (s *authServiceServer) Update(ctx context.Context, req *v1.UpdateRequest) (*v1.UpdateResponse, error) {
	if err := s.checkAPI(req.Api); err != nil {
		return nil, err
	}

	c, err := s.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	res, err := c.ExecContext(ctx, "UPDATE Auth SET `Token`=?, `Description`=? WHERE `ID`=?",
		req.Auth.Token, req.Auth.Description, req.Auth.Id)
	if err != nil {
		return nil, status.Error(codes.Unknown, "failed to update Auth-> "+err.Error())
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return nil, status.Error(codes.Unknown, "failed to retrieve rows affected value-> "+err.Error())
	}
	if rows == 0 {
		return nil, status.Errorf(codes.NotFound, "Auth with ID='%d' is not found", req.Auth.Id)
	}

	return &v1.UpdateResponse{
		Api:     apiVersion,
		Updated: rows,
	}, nil
}

// Delete todo task
func (s *authServiceServer) Delete(ctx context.Context, req *v1.DeleteRequest) (*v1.DeleteResponse, error) {
	if err := s.checkAPI(req.Api); err != nil {
		return nil, err
	}

	c, err := s.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	res, err := c.ExecContext(ctx, "DELETE FROM Auth WHERE `ID`=?", req.Id)
	if err != nil {
		return nil, status.Error(codes.Unknown, "failed to delete from Auth-> "+err.Error())
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return nil, status.Error(codes.Unknown, "failed to retrieve rows affected value-> "+err.Error())
	}
	if rows == 0 {
		return nil, status.Errorf(codes.NotFound, "Auth with ID='%d' is not found", req.Id)
	}

	return &v1.DeleteResponse{
		Api:     apiVersion,
		Deleted: rows,
	}, nil
}

// Read all todo tasks
func (s *authServiceServer) ReadAll(ctx context.Context, req *v1.ReadAllRequest) (*v1.ReadAllResponse, error) {
	if err := s.checkAPI(req.Api); err != nil {
		return nil, err
	}

	c, err := s.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	rows, err := c.QueryContext(ctx, "SELECT `ID`, `Token`, `Description` FROM Auth")
	if err != nil {
		return nil, status.Error(codes.Unknown, "failed to select from Auth-> "+err.Error())
	}
	defer rows.Close()

	var list []*v1.Auth
	for rows.Next() {
		auth := new(v1.Auth)
		if err := rows.Scan(&auth.Id, &auth.Token, &auth.Description); err != nil {
			return nil, status.Error(codes.Unknown, "failed to retrieve field values from Auth row-> "+err.Error())
		}
		list = append(list, auth)
	}

	if err := rows.Err(); err != nil {
		return nil, status.Error(codes.Unknown, "failed to retrieve data from Auth-> "+err.Error())
	}

	return &v1.ReadAllResponse{
		Api:  apiVersion,
		Auth: list,
	}, nil
}
