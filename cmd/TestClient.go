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

const ch3fTarget = "ch3f" + port
const port = ":8080"

func main() {
	host, _ := os.Hostname()

	//waiting, until cluster of peers is built
	time.Sleep(30 * time.Second)
	log.Printf("Client started as: %s", host)

	var ids [10]string
	var recipeNames = getFileNames(host)
	var contents = getContents()

	//Uploading all 10 files
	for i := 0; i < 10; i++ {
		res, recipeId, err := uploadRecipeToReplica(ch3fTarget, recipeNames[i], []byte(contents[i]))
		if err != nil || res == nil {
			log.Printf("[ERROR] Failed to upload recipe: %s", err)
			continue
		}
		if !res.Success {
			log.Printf("[INFO] Upload did not contact leader")
			log.Printf("[INFO] Redirecting to leader: %s", res.LeaderContainer)
			res, recipeId, err = uploadRecipeToReplica(res.LeaderContainer+port, recipeNames[i], []byte(contents[i]))
		}
		ids[i] = recipeId
	}

	//Waiting for the uploads to be done
	time.Sleep(15 * time.Second)

	for i := 0; i < 10; i++ {
		downloadRes, err2 := downloadRecipeFromRandomReplica(ids[i])
		if err2 != nil || downloadRes == nil || !downloadRes.Success {
			log.Printf("[INFO] Could Not download recipe with id: %s", ids[i])
			log.Printf("[Error] At downloading recipe: %v", err2)
		} else {
			log.Printf("[SUCCESS]: Downloaded Recipe. Name: %v, content:%v\n", downloadRes.Filename, string(downloadRes.Content))
		}
	}

	//select {}
}

func uploadRecipeToReplica(target string, fileName string, content []byte) (*pb.UploadResponse, string, error) {

	log.Printf("[INFO] starting uploading file: %s", fileName)

	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
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
		Filename: fileName,
		Content:  content,
	}

	log.Printf("[INFO] Upload Recipe with ID: %s and name: %s", id, req.Filename)
	res, err := client.UploadRecipe(ctx, req)
	log.Printf("[INFO] Received Response: %v", res)
	if err != nil || res == nil {
		log.Printf("[ERROR] Could not upload file with id: %s", id)
		log.Printf("[Error] Upload error: %v", err)
		return nil, "", err
	}
	return res, id, nil
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

func getFileNames(host string) [10]string {
	return [10]string{
		"chicken_wrap_" + host + ".txt",
		"vegan_burger_" + host + ".txt",
		"banana_pancake_" + host + ".txt",
		"beef_tacos_" + host + ".txt",
		"quinoa_salad_" + host + ".txt",
		"avocado_toast_" + host + ".txt",
		"spaghetti_carbonara_" + host + ".txt",
		"berry_smoothie_" + host + ".txt",
		"grilled_cheese_" + host + ".txt",
		"pumpkin_soup_" + host + ".txt",
	}
}

func getContents() [10]string {
	return [10]string{
		"Chicken, wrap, salad, sauce",
		"Plant patty, bun, lettuce, tomato",
		"Banana, oats, egg, milk",
		"Beef, taco shell, salsa, cheese",
		"Quinoa, tomato, cucumber, olive oil",
		"Avocado, toast, chili flakes",
		"Pasta, egg, bacon, cheese",
		"Berries, banana, yogurt, honey",
		"Bread, cheese, butter",
		"Pumpkin, onion, cream, spices",
	}
}
