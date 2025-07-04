package p2p

import (
	pb "ch3fs/proto"
	"context"
	"fmt"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"time"
)

type Client struct {
	server *FileServer
	logger *zap.SugaredLogger
}

func NewClient(server *FileServer, logger *zap.SugaredLogger) *Client {
	return &Client{
		server: server,
		logger: logger,
	}
}

func (c *Client) SendRecipeUploadRequest(target string, request *pb.RecipeUploadRequest) (*pb.UploadResponse, error) {
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		c.logger.Errorf("failed to connect to target %s: %v %", target, err)
		return nil, err
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
			c.logger.Errorf("error from server: %v", err)
			backoff = BackoffWithJitter(backoff)
			time.Sleep(time.Duration(backoff) * time.Millisecond)
			res, err = client.UploadRecipe(ctx, request)
			continue
		}
		break
	}

	if res != nil && !res.Success {
		c.logger.Warnf("could not write to server")
		return nil, nil
	}

	return res, nil
}

// Just needed that for the server side
func SendUpdateRecipe(target string, id uuid.UUID, seen []string) error {
	return nil
}

func ConstructRecipeUploadRequest() pb.RecipeUploadRequest {
	count := 1
	fileIdCounter := count
	filename := fmt.Sprintf("recipe%d", fileIdCounter)

	var content []byte
	recipeDesc := fmt.Sprintf("This is the recipe%d´s description", fileIdCounter)
	content = []byte(recipeDesc)
	count++
	return pb.RecipeUploadRequest{
		Filename: filename,
		Content:  content,
	}
}
