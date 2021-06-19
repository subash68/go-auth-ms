package v1

import (
	"context"
	"database/sql"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	v1 "github.com/subash68/authenticator/pkg/api/v1"
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


func (s *authServiceServer) checkAPI (api string) error {
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
	

	return &v1.ReadResponse{
		Api:  apiVersion,
		Auth: nil,
	}, nil

}

// Update todo task
func (s *authServiceServer) Update(ctx context.Context, req *v1.UpdateRequest) (*v1.UpdateResponse, error) {
	return &v1.UpdateResponse{
		Api:     apiVersion,
		Updated: 0,
	}, nil
}

// Delete todo task
func (s *authServiceServer) Delete(ctx context.Context, req *v1.DeleteRequest) (*v1.DeleteResponse, error) {
	
	return &v1.DeleteResponse{
		Api:     apiVersion,
		Deleted: 0,
	}, nil
}

// Read all todo tasks
func (s *authServiceServer) ReadAll(ctx context.Context, req *v1.ReadAllRequest) (*v1.ReadAllResponse, error) {
	return &v1.ReadAllResponse{
		Api:   apiVersion,
		Auth: nil,
	}, nil
}
