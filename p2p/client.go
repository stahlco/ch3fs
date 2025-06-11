package p2p

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

type DummyRequest struct {
	msg string
}

func SendDummyRequest(target string, request string) {
	log.Println("preparing connection...")

	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return
	}
	defer conn.Close()

	client := commpb.NewFunctionsClient(conn)
	req := &commpb.DummyReq{Msg: request}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := client.DummyTest(ctx, req)
	if err != nil {
		log.Println("Error from server", err)
		return
	}
	log.Printf("Successfully sent request, response: %s \n", res)
}
