package p2p

import (
	pb "ch3fs/proto"
	"ch3fs/storage"
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/hashicorp/memberlist"
	"log"
	"math"
	"math/rand"
	"os"
	"time"
)

const CAP = 5000.0

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
	// Checking the Context for cancellation
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if req == nil {
		return &pb.UploadResponse{Success: false}, fmt.Errorf("request cannot be nil")
	}

	// Checks before accessing the Database or/and Broadcasting
	if len(req.GetSeen()) >= 3 {
		return &pb.UploadResponse{Success: false}, fmt.Errorf("Has enough Replicas!")
	}

	// Extract and validate data from request
	filename, content := deconstructRecipeUploadRequest(req)
	if filename == "" {
		return &pb.UploadResponse{Success: false}, fmt.Errorf("filename cannot be empty")
	}
	recipeUuid, err := uuid.Parse(req.GetId())
	if err != nil {
		return &pb.UploadResponse{Success: false}, fmt.Errorf("invalid recipe ID: %w", err)
	}

	recipe := storage.NewRecipe(recipeUuid, filename, content)

	// Writing to the Database synchronously
	if err := fs.Store.StoreRecipe(ctx, recipe); err != nil {
		log.Printf("Failed to store recipe %s: %v", filename, err)
		return &pb.UploadResponse{Success: false}, fmt.Errorf("failed to store recipe: %w", err)
	}

	// Broadcasting asynchronously
	go func() {
		broadcastCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if resp, err := fs.BroadcastUploadRequest(broadcastCtx, req); err != nil || resp.Success == false {
			log.Printf("Failed to broadcast upload request for recipe %s: %v", filename, err)

		}
	}()

	return &pb.UploadResponse{Success: true}, err

}

func (fs FileServer) BroadcastUploadRequest(ctx context.Context, oldReq *pb.RecipeUploadRequest) (*pb.UploadResponse, error) {
	//TODO
	return nil, nil
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

// BackoffWithJitter calculates a random jittered backoff value
// Is public, so that remote functions can access it.
func BackoffWithJitter(backoff float64) float64 {
	backoff = math.Min(backoff*2, CAP)
	return rand.Float64() * backoff
}
