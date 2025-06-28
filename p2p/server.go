package p2p

import (
	pb "ch3fs/proto"
	"ch3fs/storage"
	"context"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/google/uuid"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/raft"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"math"
	"math/rand"
	"os"
	"slices"
	"time"
)

const CAP = 15000.0
const GRPCPort = 8080

type FileServer struct {
	pb.FileSystemServer
	Store *storage.Store
	Raft  *RaftNode
}

func NewFileServer(store *storage.Store, raft *RaftNode) *FileServer {
	return &FileServer{
		Store: store,
		Raft:  raft,
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

// UploadRecipe implements the gRPC method for uploading a recipe via Raft consensus.
func (fs *FileServer) UploadRecipe(ctx context.Context, req *pb.RecipeUploadRequest) (*pb.UploadResponse, error) {
	// checking context
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if req == nil {
		return &pb.UploadResponse{Success: false}, fmt.Errorf("UploadResponse was nil")
	}

	if fs.Raft.Raft.State() != raft.Leader {
		serverAddress, _ := fs.Raft.Raft.LeaderWithID()
		conn, err := grpc.NewClient(string(serverAddress), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("Creating New Client failed!")
		}
		defer conn.Close()
		client := pb.NewFileSystemClient(conn)
		return client.UploadRecipe(ctx, req)
	}

	// serializing request for raft consensus
	data, err := proto.Marshal(req)
	if err != nil {
		return &pb.UploadResponse{Success: false}, fmt.Errorf("failed to marshal request: %w", err)
	}

	// raft apply, new entry should be appended to log and replicated to every node
	applyFuture := fs.Raft.Raft.Apply(data, 5*time.Second)
	if err := applyFuture.Error(); err != nil {
		return &pb.UploadResponse{Success: false}, fmt.Errorf("raft apply failed: %w", err)
	}

	// response from fsm apply, can be nil or a UploadResponse
	if resp, ok := applyFuture.Response().(*pb.UploadResponse); ok {
		return resp, nil
	}

	return &pb.UploadResponse{Success: false}, nil
}

func (fs *FileServer) DownloadRecipe(ctx context.Context, req *pb.RecipeDownloadRequest) (*pb.RecipeDownloadResponse, error) {

	//Check for bad param
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if req == nil {
		return &pb.RecipeDownloadResponse{
			Success: false,
		}, fmt.Errorf("DownloadRequest was nil")
	}

	//Parsing string -> UUID
	id, err := uuid.Parse(req.GetRecipeId())
	if err != nil {
		log.Printf("Invalid UUID: %v", err)
		return nil, err
	}

	recipe, err := fs.Store.GetRecipe(ctx, id)
	if err != nil {
		log.Printf("DownloadRecipe failed with %s", err)
		return nil, err
	}

	return &pb.RecipeDownloadResponse{
		Success:  true,
		Filename: recipe.Filename,
		Content:  []byte(recipe.Content),
	}, nil
}

/*
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
			if err = fs.broadcastUpload(req, seen); err != nil {
				log.Fatalf("Broadcast Upload: %v failed with rrror: %v", recipeUuid, err)
			}
		}()
	}

	return &pb.UploadResponse{Success: true, Seen: seen}, nil
}


// Helper Functions

func (fs *FileServer) broadcastUpload(originalReq *pb.RecipeUploadRequest, seen []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var replica2, replica3 *memberlist.Node

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("broadcast timeout: %w", ctx.Err())
		default:
		}

		peers := filterPeers(fs.Peers.Members(), seen...)

		if len(peers) < 2 {
			return fmt.Errorf("insufficient amount of peers in the cluster")
		}

		replica2 = peers[rand.Intn(len(peers))]
		peers = filterPeers(peers, replica2.Name)
		if len(peers) == 0 {
			return fmt.Errorf("insufficient amount of peers in the cluster")
		}
		replica3 = peers[rand.Intn(len(peers))]

		// Seen Lists send to the Replicas
		seenReplica2 := append(seen, replica3.Name) //seen = [currentNode, replica3]
		seenReplica3 := append(seen, replica2.Name) //seen = [currentNode, replica2]

		// Channels to receive results from goroutines
		r2Channel := make(chan bool, 1)
		r3Channel := make(chan bool, 1)

		go func() {
			result := fs.handleUploadBroadcast(replica2, originalReq, seenReplica2) //includes backoff with jitter
			r2Channel <- result
		}()

		go func() {
			result := fs.handleUploadBroadcast(replica3, originalReq, seenReplica3) //includes backoff with jitter
			r3Channel <- result
		}()

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

				successfulNode := replica3

				err := fs.handlePartialFailure(ctx, successfulNode, originalReq, seen)
				if err != nil {
					return err
				}
				log.Printf("Successfully broadcasted even with partial failure")
				return nil
			}
		case r2Success && !r3Success:
			{
				seen = append(seen, replica2.Name)
				successfulNode := replica2

				err := fs.handlePartialFailure(ctx, successfulNode, originalReq, seen)
				if err != nil {
					return err
				}
				log.Printf("Successfully broadcasted even with partial failure")
				return nil
			}
		default:
			{
				continue
			}
		}
	}
}

func (fs *FileServer) handlePartialFailure(ctx context.Context, successfulNode *memberlist.Node, originalReq *pb.RecipeUploadRequest, seen []string) error {
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("broadcast timeout during recovery: %w", ctx.Err())
		default:
		}

		peers := filterPeers(fs.Peers.Members(), seen...)
		if len(peers) == 0 {
			return fmt.Errorf("no available peers for r2 replacement")
		}

		newReplica2 := peers[rand.Intn(len(peers))]
		seenNewReplica2 := append(seen, successfulNode.Name)

		if fs.handleUploadBroadcast(newReplica2, originalReq, seenNewReplica2) {
			// New r2 succeeded, now update r3's seen list and local storage
			finalSeenList := append(seen, newReplica2.Name)
			slices.Sort(finalSeenList)

			// Update successful r3 node with new seen list including new r2
			host, _, _ := net.SplitHostPort(successfulNode.Address())
			target := host + ":8080"
			recipeId, _ := uuid.Parse(originalReq.GetId())

			if err := sendUpdateRecipe(target, recipeId, finalSeenList); err != nil {
				return fmt.Errorf("failed to update seen list on successful node: %w", err)
			}

			// Update local storage
			recipe := storage.NewRecipe(recipeId, originalReq.GetFilename(), string(originalReq.GetContent()), finalSeenList)
			return fs.Store.UpdateRecipe(ctx, recipe)
		}
	}
}

// sendUpdateRecipe updates the seen[] to the one successful replica holding onto wrong seen[], after replication failed
func sendUpdateRecipe(target string, id uuid.UUID, seen []string) error {

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		var replica4 *memberlist.Node
		replica4 = filterPeers(fs.Peers.Members(), target)
		for {
			select {
			case <-ctx.Done():
				return fmt.Errorf("broadcast timeout: %w", ctx.Err())
			default:
			}
			r4Channel := make(chan bool, 1)

			go func() {
				result := fs.handleUploadBroadcast(replica4, originalReq, seenReplica2) //includes backoff with jitter
				r4Channel <- result
			}()

			r4Success := <-r4Channel
		}

	return nil
}

func (fs *FileServer) handleUploadBroadcast(targetNode *memberlist.Node, originalReq *pb.RecipeUploadRequest, seen []string) bool {
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
			backoff = BackoffWithJitter(backoff, CAP)
			time.Sleep(time.Duration(backoff) * time.Millisecond)
			continue
		}

		if err == nil && resp != nil && resp.Success {
			return true
		}
	}
}
*/

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

func deconstructRecipeUploadRequest(req *pb.RecipeUploadRequest) (string, string) {
	return req.GetFilename(), string(req.GetContent())
}

// BackoffWithJitter calculates a random jittered backoff value
// Is public, so that remote functions can access it.
func BackoffWithJitter(backoff float64, maxValue float64) float64 {
	backoff = backoff * 2
	return math.Min(rand.Float64()*backoff, maxValue)
}
