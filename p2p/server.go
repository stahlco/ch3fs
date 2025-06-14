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
	"net"
	"os"
	"slices"
	"time"
)

const CAP = 15000.0
const GRPCPort = 8080

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

func (fs *FileServer) UploadRecipe(ctx context.Context, req *pb.RecipeUploadRequest) (*pb.UploadResponse, error) {
	// Checking the Context for cancellation
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if req == nil {
		return &pb.UploadResponse{Success: false, Seen: make([]string, 0)}, fmt.Errorf("request cannot be nil")
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

	// Checks before accessing the Database or/and Broadcasting
	if len(req.GetSeen()) >= 3 {
		return &pb.UploadResponse{Success: false, Seen: req.GetSeen()}, fmt.Errorf("maximum replicas reached")
	}

	existingRecipe, err := fs.Store.GetRecipe(ctx, recipeUuid)
	if existingRecipe != nil || err != nil {
		return &pb.UploadResponse{Success: false, Seen: existingRecipe.Seen}, fmt.Errorf("recipe with uuid: %v already exists", recipeUuid)
	}

	seen := append([]string{}, req.GetSeen()...)
	seen = append(seen, fs.Peers.LocalNode().Name)
	// Sorting it for better easier comparison later
	slices.Sort(seen)

	recipe := storage.NewRecipe(recipeUuid, filename, content, seen)

	if err = fs.Store.StoreRecipe(ctx, recipe); err != nil {
		return nil, err
	}

	//broadcast asynchronously
	if len(seen) < 2 {
		go func() {
			if err = fs.BroadcastUpload(req, seen); err != nil {
				log.Fatalf("Broadcast Upload: %v failed with rrror: %v", recipeUuid, err)
			}
		}()
	}

	return &pb.UploadResponse{Success: true, Seen: seen}, nil
}

func (fs *FileServer) BroadcastUpload(originalReq *pb.RecipeUploadRequest, seen []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var replica2, replica3 *memberlist.Node
	var needReplica2, needReplica3 bool = true, true

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("broadcast timeout: %w", ctx.Err())
		default:
		}

		peers := filterPeers(fs.Peers.Members(), seen...)

		if needReplica2 && needReplica3 && len(peers) < 2 {
			return fmt.Errorf("insufficient amount of peers in the cluster")
		}

		if needReplica2 {
			replica2 = peers[rand.Intn(len(peers))]
		}

		if needReplica3 {
			peers = filterPeers(peers, replica2.Name)
			if len(peers) == 0 {
				return fmt.Errorf("insufficient amount of peers in the cluster")
			}
			replica3 = peers[rand.Intn(len(peers))]
		}

		// Seen Lists send to the Replicas
		seenReplica2 := append(seen, replica3.Name) //seen = [currentNode, replica3]
		seenReplica3 := append(seen, replica2.Name) //seen = [currentNode, replica2]

		// Channels to receive results from goroutines
		r2Channel := make(chan bool, 1)
		r3Channel := make(chan bool, 1)

		if needReplica2 {
			go func() {
				result := fs.HandleUploadBroadcast(replica2, originalReq, seenReplica2) //includes backoff with jitter
				r2Channel <- result
			}()
		} else {
			// If we already delivered to replica2 we don't care about failure
			r2Channel <- true
		}

		if needReplica3 {
			go func() {
				result := fs.HandleUploadBroadcast(replica3, originalReq, seenReplica3) //includes backoff with jitter
				r3Channel <- result
			}()
		} else {
			// If we already delivered to replica3 we don't care about failure
			r3Channel <- true
		}

		r2Success := <-r2Channel
		r3Success := <-r3Channel

		switch {
		case r2Success && r3Success:
			// Updates the Seen list in the stored Recipe
			{
				recipeSeenList := append(seen, replica2.Name, replica3.Name)
				slices.Sort(recipeSeenList)

				id, err := uuid.Parse(originalReq.GetId())
				if err != nil {
					return fmt.Errorf("parsing the UUID %v failed: %w", originalReq.GetId(), err)
				}
				recipe := storage.NewRecipe(id, originalReq.GetFilename(), string(originalReq.GetContent()), recipeSeenList)

				return fs.Store.UpdateRecipe(ctx, recipe)
			}
		case !r2Success && r3Success:
			{
				seen = append(seen, replica3.Name) // Add successful r3 to seen
				needReplica2 = true
				needReplica3 = false
				continue
			}
		case r2Success && !r3Success:
			{
				seen = append(seen, replica2.Name) // Add successful r3 to seen
				needReplica2 = false
				needReplica3 = true
				continue
			}
		default:
			// Both failed - find new r2 and r3
			{
				needReplica2 = true // Need to find new r2
				needReplica3 = true // Need to find new r3
				continue
			}
		}
	}
}

func (fs *FileServer) HandleUploadBroadcast(targetNode *memberlist.Node, originalReq *pb.RecipeUploadRequest, seen []string) bool {
	host, _, _ := net.SplitHostPort(targetNode.Address())
	target := host + ":8080"

	req := &pb.RecipeUploadRequest{
		Id:       originalReq.GetId(),
		Filename: originalReq.GetFilename(),
		Content:  originalReq.GetContent(),
		Seen:     seen,
	}
	backoff := 50.0

	for {
		resp, err := SendRecipeUploadRequest(target, req)
		if resp.Success == false {
			return false
		}
		if err != nil && ListContains(fs.Peers, targetNode) {
			backoff = BackoffWithJitter(backoff)
			time.Sleep(time.Duration(backoff) * time.Millisecond)
			continue
		}

		if err == nil && resp != nil && resp.Success {
			return true
		}
	}
}

func filterPeers(peers []*memberlist.Node, remove ...string) []*memberlist.Node {
	var nodes []*memberlist.Node
	for _, node := range peers {
		if slices.Contains(remove, node.Name) {
			continue
		}
		nodes = append(nodes, node)
	}
	return nodes
}

func (fs FileServer) DownloadRecipe(ctx context.Context, req *pb.RecipeDownloadRequest) (*pb.RecipeDownloadResponse, error) {
	return nil, nil
}

// BackoffWithJitter calculates a random jittered backoff value
// Is public, so that remote functions can access it.
func BackoffWithJitter(backoff float64) float64 {
	backoff = math.Min(backoff*2, CAP)
	return rand.Float64() * backoff
}

func deconstructRecipeUploadRequest(req *pb.RecipeUploadRequest) (string, string) {
	return req.GetFilename(), string(req.GetContent())
}
