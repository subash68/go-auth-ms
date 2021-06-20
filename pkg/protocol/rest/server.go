package rest

import (
	"context"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	v1 "github.com/subash68/authenticator/pkg/api/v1"
	"google.golang.org/grpc"
)

func RunServer(ctx context.Context, grpcPort, httpPort string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	mux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithInsecure()}

	//this should be from gw file in pb
	if err := v1.RegisterAuthServiceHandlerFromEndpoint(ctx, mux, "localhost:" +grpcPort, opts); err != nil {

	}

	return nil
}