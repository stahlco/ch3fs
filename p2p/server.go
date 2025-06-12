package p2p

import (
	pb "ch3fs/proto"
	"ch3fs/storage"
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/hashicorp/memberlist"
	"log"
	"os"
	"time"
)

type FileServer struct {
	pb.FileSystemServer
	Store *storage.Store
	Peers *memberlist.Memberlist
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

	// Fetching Data from Request
	filename, content := deconstructRecipeUploadRequest(req)
	recipeUuid, _ := uuid.Parse(req.GetId())
	recipe := storage.NewRecipe(recipeUuid, filename, content)

	// Checks before accessing the Database or/and Broadcasting
	if len(req.GetSeen()) >= 3 {
		return &pb.UploadResponse{Success: false}, fmt.Errorf("Has enough Replicas!")
	}

	// Writing to the Database asynchronous
	errChan := make(chan error)
	go func() {
		errChan <- fs.Store.StoreRecipe(ctx, recipe)
	}()

	err := <-errChan
	if err != nil {
		resp := pb.UploadResponse{
			Success: false,
		}
		log.Fatalf("Failed to store the file: %s with Eroor: %v", filename, err)
		return &resp, err
	}

	broadcastChan := make(chan *pb.UploadResponse)
	go func() {
		broadcastResponse, err := fs.BroadcastUploadRequest(req)
		if err != nil {
			log.Fatalf("Not able to broadcast the msg!")
		}

		broadcastChan <- broadcastResponse
	}()

	resp := pb.UploadResponse{
		Success: true,
	}
	return &resp, err

}

func (fs FileServer) BroadcastUploadRequest(oldReq *pb.RecipeUploadRequest) (*pb.UploadResponse, error) {
	req := fs.createBroadcastRequest(oldReq)

	broadcastCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	//TODO
}

func (fs FileServer) createBroadcastRequest(oldReq *pb.RecipeUploadRequest) *pb.RecipeUploadRequest {
	return &pb.RecipeUploadRequest{
		Id:       oldReq.GetId(),
		Filename: oldReq.GetFilename(),
		Content:  []byte(oldReq.GetContent()),
		Seen:     append(oldReq.GetSeen(), fs.Peers.LocalNode().Name), //Adds the Local Node to seen
	}
}

func deconstructRecipeUploadRequest(req *pb.RecipeUploadRequest) (string, string) {
	return req.GetFilename(), string(req.GetContent())
}
