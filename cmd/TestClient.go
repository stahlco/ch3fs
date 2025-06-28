package cmd

import (
	pb "ch3fs/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"time"
)

const ch3fTarget = "ch3f:8080"

func main() {

	time.Sleep(100)
	res, err := uploadRecipeToRandomReplica()

	if !res.Success {
		log.Println("Could not write to datastore")
		log.Printf("error from server: %v", err)
	} else {
		log.Println("successfully stored recipe")
	}

	time.Sleep(50)

	res2, err2 := downloadRecipeFromRandomReplica()
	if !res2.Success {
		log.Println("Could not read from datastore")
		log.Printf("error from server : %v", err2)
	} else {
		log.Printf("successfully read recipe: %v, %v\n", res2.Filename, string(res2.Content))
	}

	select {}
}

func uploadRecipeToRandomReplica() (*pb.UploadResponse, error) {

	log.Println("preparing connection...")

	conn, err := grpc.NewClient(ch3fTarget, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Creating New Client failed!")
	}
	defer conn.Close()
	client := pb.NewFileSystemClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &pb.RecipeUploadRequest{
		Id:       "test-001",
		Filename: "recipe1.txt",
		Content:  []byte("This is the recipe1 description"),
	}

	return client.UploadRecipe(ctx, req)
}

func downloadRecipeFromRandomReplica() (*pb.RecipeDownloadResponse, error) {

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
		RecipeId: "test-001",
	}

	return client.DownloadRecipe(ctx, req)

}
