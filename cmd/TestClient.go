package main

import (
	pb "ch3fs/proto"
	"github.com/google/uuid"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"time"
)

const ch3fTarget = "ch3f:8080"

func main() {

	time.Sleep(30 * time.Second)
	res, id, err := uploadRecipeToRandomReplica()

	if err != nil || res == nil || !res.Success {
		log.Println("Could not write to datastore")
		log.Printf("error from server: %v", err)
	} else {
		log.Println("successfully stored recipe")
	}

	time.Sleep(5 * time.Second)

	res2, err2 := downloadRecipeFromRandomReplica(id)

	if err2 != nil || res2 == nil || !res2.Success {
		log.Println("Could not read from datastore")
		log.Printf("error from server : %v", err2)
	} else {
		log.Printf("successfully read recipe: %v, %v\n", res2.Filename, string(res2.Content))
	}

	select {}
}

func uploadRecipeToRandomReplica() (*pb.UploadResponse, string, error) {

	log.Println("preparing connection...")

	conn, err := grpc.NewClient(ch3fTarget, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Creating New Client failed!")
	}
	defer conn.Close()
	client := pb.NewFileSystemClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	id := uuid.New().String()

	req := &pb.RecipeUploadRequest{
		Id:       id,
		Filename: "recipe1.txt",
		Content:  []byte("This is the recipe1 description"),
	}

	res, err2 := client.UploadRecipe(ctx, req)
	return res, id, err2
}

func downloadRecipeFromRandomReplica(id string) (*pb.RecipeDownloadResponse, error) {

	log.Println("preparing connection...")

	conn, err := grpc.NewClient(ch3fTarget, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Creating New Client failed!")
	}
	defer conn.Close()
	client := pb.NewFileSystemClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &pb.RecipeDownloadRequest{
		RecipeId: id,
	}

	return client.DownloadRecipe(ctx, req)

}
