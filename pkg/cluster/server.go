package cluster

import (
	"ch3fs/pkg/storage"
	pb "ch3fs/proto"
	"context"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/google/uuid"
	_ "github.com/hashicorp/golang-lru"
	lru "github.com/hashicorp/golang-lru"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/raft"
	"github.com/shirou/gopsutil/v3/cpu"
	"go.uber.org/zap"
	"log"
	"math"
	"math/rand"
	"net"
	"time"
)

const CAP = 15000.0
const GRPCPort = ":8080"

type FileServer struct {
	pb.FileSystemServer
	Store  *storage.Store
	Raft   *RaftNode
	logger *zap.Logger
	Cache  *lru.ARCCache //Cache, that tracks recent usage
}

func NewFileServer(raft *RaftNode, logger *zap.Logger, cache *lru.ARCCache) *FileServer {
	return &FileServer{
		Store:  raft.Store,
		logger: logger,
		Raft:   raft,
		Cache:  cache,
	}
}

// UploadRecipe handles a gRPC request to upload a recipe using Raft consensus.
//
// This method performs several responsibilities:
//   - Validates the request context and input.
//   - Implements load shedding based on real-time CPU usage
//   - If the server is not the current Raft leader, it looks up and returns
//     the IP address of the current leader to the client.
//   - If the server is the leader, it serializes the request and applies it
//     to the Raft log, ensuring the Write is replicated across the cluster.
//
// Returns:
//   - *pb.UploadResponse with Success = true if the recipe was replicated successfully.
//   - *pb.UploadResponse with Success = false and a leader redirect IP if the current node is not the leader.
//   - An error if CPU-based shedding is triggered or internal failures occur.
//
// Load Shedding Logic:
//   - CPU > 90%: Hard reject (priority-based).
//   - CPU > 80%: 25% chance of rejection.
//   - CPU > 75%: 10% chance of rejection.
//   - CPU > 70%: 5% chance of rejection.
//
// Errors returned may include:
//   - Context cancellation
//   - CPU load shedding
//   - Raft apply failures
//   - Leader IP resolution failures
func (fs *FileServer) UploadRecipe(ctx context.Context, req *pb.RecipeUploadRequest) (*pb.UploadResponse, error) {
	log.Printf("Received upload request")
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if req == nil {
		return &pb.UploadResponse{Success: false}, fmt.Errorf("UploadResponse was nil")
	}

	//CPU Information for Load Shedding
	usage, err := cpu.Percent(time.Second, false)
	if err != nil {
		return nil, fmt.Errorf("not able to get CPU data: %v", err)
	}

	//Priority Load Shedding
	if usage[0] > 90 {
		fs.logger.Info("Upload Request will be shed based on cpu threshold", zap.Float64("threshold", usage[0]), zap.Any("request", req))
		return &pb.UploadResponse{Success: false}, fmt.Errorf("request been shedded based on our priority load shedding requirements")
	}

	//Probablistic Load Shedding
	if usage[0] > 80 && rand.Intn(4)%4 == 0 {
		fs.logger.Info("Upload Request will be shed based on probabilistic (25%)", zap.Float64("threshold", usage[0]))
		return &pb.UploadResponse{Success: false}, fmt.Errorf("request been shedded based on our probablistic load shedding requirements")
	}

	if usage[0] > 75 && rand.Intn(10)%10 == 0 {
		fs.logger.Info("Upload Request will be shed based on probabilistic (10%)", zap.Float64("threshold", usage[0]))
		return &pb.UploadResponse{Success: false}, fmt.Errorf("request been shedded based on our priority load shedding requirements")
	}

	if usage[0] > 70 && rand.Intn(20)%20 == 0 {
		fs.logger.Info("Upload Request will be shed based on probabilistic (5%)", zap.Float64("threshold", usage[0]))
		return &pb.UploadResponse{Success: false}, fmt.Errorf("request been shedded based on our priority load shedding requirements")
	}

	if fs.Raft.Raft.State() != raft.Leader {

		_, leaderId := fs.Raft.Raft.LeaderWithID()

		leaders, err := net.LookupIP(string(leaderId))
		if err != nil || len(leaders) == 0 {
			log.Printf("could not resolve IP for leader %s: %v", leaderId, err)
			return &pb.UploadResponse{Success: false}, fmt.Errorf("could not resolve leader host")
		}
		leaderIP := leaders[0].String()

		return &pb.UploadResponse{
			Success:         false,
			LeaderContainer: leaderIP,
		}, nil
	}

	// serializing request for raft consensus
	data, err := proto.Marshal(req)
	if err != nil {
		return &pb.UploadResponse{Success: false}, fmt.Errorf("failed to marshal request: %w", err)
	}

	// raft apply, new entry should be appended to log and replicated to every node
	log.Printf("applying the recipe to raft: %s", req.Filename)
	applyFuture := fs.Raft.Raft.Apply(data, 5*time.Second)
	if err := applyFuture.Error(); err != nil {
		return &pb.UploadResponse{Success: false}, fmt.Errorf("raft apply failed: %w", err)
	}

	// response from fsm apply, can be nil or a UploadResponse
	log.Printf("response of upload %s : %v", req.Filename, applyFuture.Response())
	if resp, ok := applyFuture.Response().(*pb.UploadResponse); ok {

		id, err := uuid.Parse(req.Id)
		if err != nil {
			fs.logger.Error("Failed to parse UUID", zap.Error(err))
		}
		fs.Cache.Add(req.Filename, storage.Recipe{
			RecipeId: id,
			Filename: req.Filename,
			Content:  string(req.Content),
		})
		return resp, nil
	}

	return &pb.UploadResponse{Success: false}, nil
}

// DownloadRecipe handles a gRPC request to retrieve a recipe by ID from persistent storage.
//
// This method performs the following operations:
//   - Validates the incoming context and request parameters.
//   - Implements probabilistic load shedding based on current CPU usage to avoid overload.
//   - Checks cache, if request produces a cache miss it performs load shedding again.
//   - Attempts to retrieve the recipe from the backend store.
//   - Writes the data to the cache before returning.
//
// Load Shedding Behavior:
//   - Applies probabilistic rejection at high CPU usage levels (not priority because Download has the highest priority):
//   - CPU > 95%: 25% chance of rejection
//   - CPU > 90%: 10% chance of rejection
//   - CPU > 85%: 5% chance of rejection
//
// Returns:
//   - *pb.RecipeDownloadResponse with Success = true and the recipe data on success.
//   - *pb.RecipeDownloadResponse with Success = false and an error message on failure.
//
// Errors returned may include:
//   - Context cancellation
//   - CPU-based probabilistic shedding
//   - Invalid or unparsable UUID
//   - Recipe not found or store retrieval error
func (fs *FileServer) DownloadRecipe(ctx context.Context, req *pb.RecipeDownloadRequest) (*pb.RecipeDownloadResponse, error) {
	log.Printf("Received download request")
	//Check for bad param
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if req == nil {
		return &pb.RecipeDownloadResponse{
			Success: false,
		}, fmt.Errorf("DownloadRequest was nil")
	}

	usage, err := cpu.Percent(time.Second, false)
	if err != nil {
		return nil, fmt.Errorf("not able to get CPU data: %v", err)
	}

	//Probabilistic Load Shedding
	if usage[0] > 95 && rand.Intn(4)%4 == 0 {
		fs.logger.Info("Download Request will be shed based on probabilistic (25%)", zap.Float64("treshold", usage[0]))
		return &pb.RecipeDownloadResponse{Success: false}, fmt.Errorf("request been shedded based on our priority load shedding requirements")
	}

	if usage[0] > 90 && rand.Intn(10)%10 == 0 {
		fs.logger.Info("Download Request will be shed based on probabilistic (10%)", zap.Float64("treshold", usage[0]))
		return &pb.RecipeDownloadResponse{Success: false}, fmt.Errorf("request been shedded based on our priority load shedding requirements")
	}

	if usage[0] > 85 && rand.Intn(20)%20 == 0 {
		fs.logger.Info("Download Request will be shed based on probabilistic (5%)", zap.Float64("treshold", usage[0]))
		return &pb.RecipeDownloadResponse{Success: false}, fmt.Errorf("request been shedded based on our priority load shedding requirements")
	}

	//Parsing string -> UUID
	id, err := uuid.Parse(req.GetRecipeId())
	if err != nil {
		fs.logger.Error("Invalid UUID received", zap.Any("uuid", id), zap.Error(err))
		return nil, err
	}

	//Checking, if the recipe is cached
	cachedValueRaw, isInCache := fs.Cache.Get(req.RecipeId)
	if isInCache {
		recipe, succ := cachedValueRaw.(*storage.Recipe)
		if succ {
			fs.logger.Info("Successfully downloaded recipe out of cache")
			return &pb.RecipeDownloadResponse{
				Success:  true,
				Filename: recipe.Filename,
				Content:  []byte(recipe.Content),
			}, nil
		} else {
			fs.logger.Error("Could not parse Recipe in cache")
		}
	}

	//If Cache Miss, do a Load Shed -> Measure metrics again, and check the remaining time

	recipe, err := fs.Store.GetRecipe(ctx, id)
	if err != nil {
		log.Printf("DownloadRecipe failed with %s", err)
		return nil, err
	}

	//Adding recipe to cache
	//if the cache is full, the least recently used is deleted automatically
	fs.Cache.Add(recipe.RecipeId, recipe)

	return &pb.RecipeDownloadResponse{
		Success:  true,
		Filename: recipe.Filename,
		Content:  []byte(recipe.Content),
	}, nil
}

// BackoffWithJitter calculates a random jittered backoff value
// Is public, so that remote functions can access it.
func BackoffWithJitter(backoff float64) float64 {
	backoff = backoff * 2
	return math.Min(rand.Float64()*backoff, CAP)
}

// ListContainsContainerName checks if the memberlist of the cluster contains a given container name
func ListContainsContainerName(l *memberlist.Memberlist, container string) bool {
	for _, m := range l.Members() {
		if m.Name == container {
			return true
		}
	}
	return false
}
