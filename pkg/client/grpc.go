package client

import (
	pb "ch3fs/proto"
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"time"
)

const ch3fTarget = "ch3f:8080"

func (c *Client) UploadRecipe(target string, filename string, content []byte) (*pb.UploadResponse, error) {
	backoff := 50.0
	for {
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
		if err != nil {
			backoffBefore := backoff
			backoff = BackoffWithJitter(backoff)
			time.Sleep(time.Duration(backoff) * time.Millisecond)

			if backoffBefore == backoff {
				return nil, err
			}
			log.Printf("Now retrying with backoff: %fms", backoff)
			continue
		}

		return res, err
	}
}

func (c *Client) DownloadRecipe(filename string) (*pb.RecipeDownloadResponse, error) {
	backoff := 50.0

	for {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		conn, err := grpc.NewClient(c.RoundRobin.Next(), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, err
		}

		client := pb.NewFileSystemClient(conn)

		c.Logger.Infof("Sending Download Request to target: %s", conn.Target())
		req := &pb.RecipeDownloadRequest{Filename: filename}

		res, err := client.DownloadRecipe(ctx, req)
		if err != nil {
			backoffBefore := backoff
			backoff = BackoffWithJitter(backoff)
			time.Sleep(time.Duration(backoff) * time.Millisecond)

			if backoffBefore == backoff {
				return nil, err
			}
			log.Printf("Now retrying with backoff: %fms", backoff)
			continue
		}

		return res, nil
	}
}
