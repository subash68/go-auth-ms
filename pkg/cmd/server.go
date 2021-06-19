package cmd

import (
	"context"
	"database/sql"
	"flag"
	"fmt"

	_ "github.com/go-sql-driver/mysql"

	"github.com/subash68/authenticator/pkg/protocol/grpc"
	v1 "github.com/subash68/authenticator/pkg/service/v1"
)


type Config struct {
	GRPCPort string

	DatastoreDBHost string
	DatastoreDBUser string
	DatastoreDBPassword string
	DatastoreDBSchema string
}

func RunServer() error {
	ctx := context.Background()

	var cfg Config

	flag.StringVar(&cfg.GRPCPort, "grpc-port", "", "gRPC port to bind")
	flag.StringVar(&cfg.DatastoreDBHost, "db-host", "", "Database host")
	flag.StringVar(&cfg.DatastoreDBUser, "db-user", "", "Database user")
	flag.StringVar(&cfg.DatastoreDBPassword, "db-password", "", "Database password")
	flag.StringVar(&cfg.DatastoreDBSchema, "db-schema", "", "Database schema")
	flag.Parse()

	if len(cfg.GRPCPort) == 0 {
		return fmt.Errorf("Invalid TCP port for gRPC server: '%s'", cfg.GRPCPort)
	}

	param := "parseTime=true"

	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?%s",
	cfg.DatastoreDBUser,
	cfg.DatastoreDBPassword,
	cfg.DatastoreDBHost,
	cfg.DatastoreDBSchema,
	param)

	db, err := sql.Open("mysql", dsn)

	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}

	defer db.Close()

	v1API := v1.NewAuthServiceServer(db)

	return grpc.RunServer(ctx, v1API, cfg.GRPCPort)
}