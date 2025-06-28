package p2p

import (
	pb "ch3fs/proto"
	"context"
	"fmt"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"time"
)

type Client struct {
	server *FileServer
}

func NewClient(server *FileServer) *Client {
	return &Client{
		server: server,
	}
}

//TODO file check <1mb

type DummyRequest struct {
	msg string
}

func SendRecipeUploadRequest(target string, request *pb.RecipeUploadRequest) (*pb.UploadResponse, error) {
	log.Println("preparing connection...")

	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Creating New Client failed!")
	}
	defer conn.Close()
	client := pb.NewFileSystemClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := client.UploadRecipe(ctx, request)

	//Retrying with Jitter, until the Client has uploaded the recipe:
	backoff := 50.0
	for {
		if err != nil {
			log.Println("Error from server", err)
			backoff = BackoffWithJitter(backoff, 10000)
			time.Sleep(time.Duration(backoff) * time.Millisecond)
			res, err = client.UploadRecipe(ctx, request)
			continue
		}
	}

	if !res.Success {
		log.Println("Could not write to datastore")
		return nil, nil
	}

	return res, nil
}

// Just needed that for the server side
func SendUpdateRecipe(target string, id uuid.UUID, seen []string) error {
	return nil
}

func Join(target string, id string, addr string) (*pb.JoinResponse, error) {
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Not able to create a grpc client for target: %s", target)
	}
	defer conn.Close()

	client := pb.NewRaftClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &pb.JoinRequest{
		Id:   id,
		Addr: addr,
	}
	log.Printf("Now sending join request to server")

	res, err := client.Join(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("joining cluster of server failed with error: %s", err)
	}
	return res, nil
}

func ConstructRecipeUploadRequest() pb.RecipeUploadRequest {
	count := 1
	fileIdCounter := count
	filename := fmt.Sprintf("recipe%d", fileIdCounter)

	var content []byte
	recipeDesc := fmt.Sprintf("This is the recipe%d´s description", fileIdCounter)
	content = []byte(recipeDesc)
	count++
	return pb.RecipeUploadRequest{Filename: filename, Content: content}
}

func SendDummyRequest(target string, request *pb.DummyTestRequest) {
	log.Println("preparing connection...")

	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return
	}
	defer conn.Close()

	client := pb.NewFileSystemClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := client.DummyTest(ctx, request)
	if err != nil {
		log.Println("Error from server", err)
		return
	}
	log.Printf("Successfully sent DummyRequest, response: %s \n", res)
}
