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
		return &pb.UploadResponse{Success: false, Seen: make([]string, 0)}, fmt.Errorf("request cannot be nil")
	}

	// Checks before accessing the Database or/and Broadcasting
	if len(req.GetSeen()) >= 3 {
		return &pb.UploadResponse{Success: false, Seen: make([]string, 0)}, fmt.Errorf("maximum replicas reached")
	}

	// Extract and validate data from request
	filename, content := deconstructRecipeUploadRequest(req)
	if filename == "" {
		return &pb.UploadResponse{Success: false, Seen: make([]string, 0)}, fmt.Errorf("filename cannot be empty")
	}

	recipeUuid, err := uuid.Parse(req.GetId())
	if err != nil {
		return &pb.UploadResponse{Success: false, Seen: make([]string, 0)}, fmt.Errorf("invalid recipe ID: %w", err)
	}

	existingRecipe, err := fs.Store.GetRecipe(ctx, recipeUuid)
	if existingRecipe != nil && err == nil {
		// Update seen nodes if current node is not in the list
		updatedSeen := make([]string, len(existingRecipe.Seen))
		copy(updatedSeen, existingRecipe.Seen)

		nodeExists := false
		for _, node := range updatedSeen {
			if node == fs.Peers.LocalNode().Name {
				nodeExists = true
				break
			}
		}

		if !nodeExists {
			updatedSeen = append(updatedSeen, fs.Peers.LocalNode().Name)

			existingRecipe.Seen = updatedSeen

			// Update need to be implemented
			if updateErr := fs.Store.UpdateRecipe(ctx, existingRecipe); updateErr != nil {
				log.Printf("Failed to update seen nodes for existing recipe %v: %v", recipeUuid, updateErr)
			}
		}
		log.Printf("Recipe: %v exists in Database with Seen: %v", recipeUuid, existingRecipe.Seen)
		return &pb.UploadResponse{Success: false, Seen: existingRecipe.Seen}, err
	} else if err != nil {
		return &pb.UploadResponse{Success: false, Seen: make([]string, 0)}, err
	}

	// Create a new recipe with the current node in the seen list
	updatedSeen := make([]string, len(req.GetSeen()))
	copy(updatedSeen, req.GetSeen())

	nodeExists := false
	for _, node := range updatedSeen {
		if node == fs.Peers.LocalNode().Name {
			nodeExists = true
			break
		}
	}
	if !nodeExists {
		updatedSeen = append(updatedSeen, fs.Peers.LocalNode().Name)
	}

	recipe := storage.NewRecipe(recipeUuid, filename, content)
	recipe.Seen = updatedSeen

	// Store recipe in database
	if err := fs.Store.StoreRecipe(ctx, recipe); err != nil {
		log.Printf("Failed to store recipe %s: %v", filename, err)
		return &pb.UploadResponse{Success: false, Seen: make([]string, 0)}, fmt.Errorf("failed to store recipe: %w", err)
	}

	// Start async broadcast if we haven't reached replica limit
	if len(updatedSeen) < 3 {
		go fs.handleAsyncBroadcast(req, updatedSeen)
	}

	return &pb.UploadResponse{Success: true, Seen: updatedSeen}, nil
}

func (fs FileServer) DownloadRecipe(ctx context.Context, req *pb.RecipeDownloadRequest) (*pb.RecipeDownloadResponse, error) {
	return nil, nil
}

func (fs FileServer) handleAsyncBroadcast(originalReq *pb.RecipeUploadRequest, seen []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	broadcastReq := &pb.RecipeUploadRequest{
		Id:       originalReq.GetId(),
		Filename: originalReq.GetFilename(),
		Content:  originalReq.GetContent(),
		Seen:     seen,
	}

	resp, err := fs.BroadcastUploadRequest(ctx, broadcastReq)
	if err != nil {
		log.Printf("Failed to broadcast upload request for recipe %s: %v", originalReq.GetFilename(), err)
		return
	}

	if resp != nil && resp.Success && len(resp.Seen) > len(seen) {
		id, _ := uuid.Parse(originalReq.GetId())

		storeRecipe := storage.Recipe{
			RecipeId: id,
			Filename: originalReq.GetFilename(),
			Content:  string(originalReq.GetContent()),
			Seen:     resp.GetSeen(),
		}

		// Update database with final seen list from broadcast
		if err := fs.Store.UpdateRecipe(ctx, &storeRecipe); err != nil {
			log.Printf("Failed to update seen nodes for recipe %s after broadcast: %v", originalReq.GetFilename(), err)
		} else {
			log.Printf("Successfully updated recipe %s with final seen nodes: %v", originalReq.GetFilename(), resp.Seen)
		}
	}
}

func (fs FileServer) BroadcastUploadRequest(ctx context.Context, oldReq *pb.RecipeUploadRequest) (*pb.UploadResponse, error) {
	// TODO
	return nil, nil
}

// BackoffWithJitter calculates a random jittered backoff value
// Is public, so that remote functions can access it.
func BackoffWithJitter(backoff float64) float64 {
	backoff = math.Min(backoff*2, CAP)
	return rand.Float64() * backoff
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
