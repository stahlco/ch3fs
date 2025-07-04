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
	"go.uber.org/zap"
	"log"
	"math"
	"math/rand"
	"net"
	"slices"
	"time"
)

const CAP = 15000.0
const GRPCPort = 8080

type FileServer struct {
	pb.FileSystemServer
	Store  *storage.Store
	Raft   *RaftNode
	logger *zap.Logger
}

func NewFileServer(store *storage.Store, raft *RaftNode, logger zap.Logger) *FileServer {
	return &FileServer{
		Store: store,
		Raft:  raft,
	}
}

// UploadRecipe implements the gRPC method for uploading a recipe via Raft consensus.
func (fs *FileServer) UploadRecipe(ctx context.Context, req *pb.RecipeUploadRequest) (*pb.UploadResponse, error) {
	// checking context
	log.Printf("Uploading recipe received: %s ", req.Filename)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if req == nil {
		return &pb.UploadResponse{Success: false}, fmt.Errorf("UploadResponse was nil")
	}

	if fs.Raft.Raft.State() != raft.Leader {

		_, leaderId := fs.Raft.Raft.LeaderWithID()

		leaders, err := net.LookupIP(string(leaderId))
		if err != nil || len(leaders) == 0 {
			log.Printf("could not resolve IP for leader %s: %v", leaderId, err)
			return &pb.UploadResponse{Success: false}, fmt.Errorf("could not resolve leader host")
		}
		leaderIP := leaders[0].String()

		//TODO return the ip address
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
func BackoffWithJitter(backoff float64) float64 {
	backoff = backoff * 2
	return math.Min(rand.Float64()*backoff, CAP)
}

func ListContainsContainerName(l *memberlist.Memberlist, container string) bool {
	for _, m := range l.Members() {
		if m.Name == container {
			return true
		}
	}
	return false
}
