package client

import (
	pb "ch3fs/proto"
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"time"
)

const ch3fTarget = "ch3f:8080"

func (c *Client) UploadRecipe(target string, filename string, content []byte) (*pb.UploadResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil
	}

	client := pb.NewFileSystemClient(conn)

	req := &pb.RecipeUploadRequest{
		Filename: filename,
		Content:  content,
	}
	res, err := client.UploadRecipe(ctx, req)
	return res, err
}

// Retries with Backoff

func (c *Client) DownloadRecipe(filename string) (*pb.RecipeDownloadResponse, error) {

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(c.RoundRobin.Next(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil
	}

	client := pb.NewFileSystemClient(conn)

	c.Logger.Infof("Sending Download Request to target: %s", conn.Target())
	req := &pb.RecipeDownloadRequest{Filename: filename}
	return client.DownloadRecipe(ctx, req)
}
