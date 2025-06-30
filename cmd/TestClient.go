package main

import (
	pb "ch3fs/proto"
	"github.com/google/uuid"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"os"
	"time"
)

const ch3fTarget = "ch3f:8080"
const port = ":8080"

func main() {
	host, _ := os.Hostname()

	log.Printf("Client started as: %s ", host)
	time.Sleep(30 * time.Second)
	uploadRes, recipeId, err := uploadRecipeToReplica(ch3fTarget)

	if err != nil || uploadRes == nil {
		log.Println("Could not write to datastore")
		log.Printf("error from server: %v", err)
	} else {
		if !uploadRes.Success {
			log.Println("did not contact the leader")
			log.Printf("the current leader is: %v", uploadRes.LeaderContainer)
			uploadRes, recipeId, err = uploadRecipeToReplica(uploadRes.LeaderContainer + port)
		}
	}

	time.Sleep(5 * time.Second)

	downloadRes, err2 := downloadRecipeFromRandomReplica(recipeId)

	if err2 != nil || downloadRes == nil || !downloadRes.Success {
		log.Println("Could not read from datastore")
		log.Printf("error from server : %v", err2)
	} else {
		log.Printf("successfully read recipe: %v, %v\n", downloadRes.Filename, string(downloadRes.Content))
	}

	select {}
}

func uploadRecipeToReplica(replica string) (*pb.UploadResponse, string, error) {

	conn, err := grpc.NewClient(replica, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Creating New Client failed!")
	}
	defer conn.Close()

	log.Printf("[INFO] Connection established with: %s", conn.Target())

	client := pb.NewFileSystemClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	id := uuid.New().String()

	req := &pb.RecipeUploadRequest{
		Id:       id,
		Filename: "recipe1.txt",
		Content:  []byte("This is the recipe1 description"),
	}

	log.Printf("[INFO] Upload Recipe with ID: %s", id)
	res, err2 := client.UploadRecipe(ctx, req)

	log.Printf("[INFO] Received Response: %v", res)
	return res, id, err2
}

func downloadRecipeFromRandomReplica(id string) (*pb.RecipeDownloadResponse, error) {

	conn, err := grpc.NewClient(ch3fTarget, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Creating New Client failed!")
	}
	defer conn.Close()
	log.Printf("[INFO] Connection established with: %s", conn.Target())

	client := pb.NewFileSystemClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &pb.RecipeDownloadRequest{
		RecipeId: id,
	}

	log.Printf("[INFO] Download Recipe with ID: %s", id)
	return client.DownloadRecipe(ctx, req)

}
