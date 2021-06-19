package main

import (
	"context"
	"flag"
	"log"
	"time"

	v1 "github.com/subash68/authenticator/pkg/api/v1"
	"google.golang.org/grpc"
)


const (
	apiVersion = "v1"
)

func main() {
	address := flag.String("server", "", "gRPC server in format host:port")

	flag.Parse()

	con, err := grpc.Dial(*address, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}

	defer con.Close()

	c := v1.NewAuthServiceClient(con)

	ctx, cancel := context.WithTimeout(context.Background(),5*time.Second)
	defer cancel()

	t := time.Now().In(time.UTC)
	pfx := t.Format(time.RFC3339Nano)

	req1 := v1.CreateRequest{
		Api: apiVersion,
		Auth: &v1.Auth{
			Token: "Test token to be stored(" + pfx + ")",
			Description: "Test description to be stored(" + pfx + ")",
		},
	}

	res1, err := c.Create(ctx, &req1)
	if err != nil {
		log.Fatalf("Create failed: %v", err)
	}
	log.Printf("Create result: <%v>\n\n", res1)
	

}