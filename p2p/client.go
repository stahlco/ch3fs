package p2p

import (
	pb "ch3fs/proto"
	"context"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"time"
)

type Client struct {
}

//TODO file check <1mb

type DummyRequest struct {
	msg string
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
	if err != nil {
		log.Println("Error from server", err)
		return nil, err
	}
	if !res.Success {
		log.Println("Could not write to datastore")
		return nil, nil
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
