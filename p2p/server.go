package p2p

import (
	pb "ch3fs/proto"
	"ch3fs/storage"
	"context"
	"fmt"
	"github.com/google/uuid"
	"log"
	"os"
)

type FileServer struct {
	pb.FileSystemServer
	Store *storage.Store
}

func NewFileServer(store *storage.Store) *FileServer {
	return &FileServer{
		Store: store,
	}
}

func (fs FileServer) DummyTest(ctx context.Context, req *pb.DummyTestRequest) (*pb.DummyTestResponse, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	hn, _ := os.Hostname()
	// do smth
	resp := pb.DummyTestResponse{Msg: fmt.Sprintf("%s and I am Server: %s, ", req.GetMsg(), hn)}
	return &resp, nil
}

func (fs FileServer) UploadRecipe(ctx context.Context, req *pb.RecipeUploadRequest) (*pb.UploadResponse, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	filename, content := deconstructRecipeUploadRequest(req)
	recipeUuid, _ := uuid.Parse(req.GetId())
	recipe := storage.NewRecipe(recipeUuid, filename, content)

	err := fs.Store.StoreRecipe(ctx, recipe)
	if err != nil {
		resp := pb.UploadResponse{
			Success: false,
		}
		log.Fatalf("Failed to store the file: %s with Eroor: %v", filename, err)
		return &resp, err
	}

	resp := pb.UploadResponse{
		Success: true,
	}
	return &resp, err

}

func deconstructRecipeUploadRequest(req *pb.RecipeUploadRequest) (string, string) {
	return req.GetFilename(), string(req.GetContent())
}
