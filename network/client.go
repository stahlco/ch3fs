package network

import (
	commpb "ch3fs/proto"
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"time"
)

type Client struct {
}

type dummyRequest struct {
	msg string
}

func sendDummyRequest(request string) {
	log.Println("preparing connection...")

	target := "localhost:8080" //random container
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return
	}
	defer func(conn *grpc.ClientConn) {
		err := conn.Close()
		if err != nil {
			log.Println("error closing the connection:", err)
		}
	}(conn)

	client := commpb.NewFunctionsClient(conn)
	req := &commpb.DummyReq{Msg: request}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	res, err := client.DummyTest(ctx, req)
	if err != nil {
		log.Println("Error from server", err)
		return
	}
	log.Printf("successfully sent request, response: %s \n", res)
}
