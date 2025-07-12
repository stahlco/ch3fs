package transport

import (
	"ch3fs/pkg/cluster/download"
	"ch3fs/pkg/cluster/loadshed"
	"ch3fs/pkg/cluster/upload"
	pb "ch3fs/proto"
	"context"
	"fmt"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/raft"
	"go.uber.org/zap"
	"google.golang.org/grpc/peer"
	"log"
	"math"
	"math/rand"
)

const GRPCPort = ":8080"

type FileServer struct {
	pb.FileSystemServer
	UploadQueue   *upload.Queue
	DownloadQueue *download.Queue
	logger        *zap.SugaredLogger
	Raft          *raft.Raft
}

func NewFileServer(uq *upload.Queue, dq *download.Queue, raft *raft.Raft, logger *zap.SugaredLogger) *FileServer {
	return &FileServer{
		UploadQueue:   uq,
		DownloadQueue: dq,
		Raft:          raft,
		logger:        logger,
	}
}

func (fs *FileServer) UploadRecipe(ctx context.Context, req *pb.RecipeUploadRequest) (*pb.UploadResponse, error) {
	p, ok := peer.FromContext(ctx)
	if ok {
		log.Printf("Received Upload Request from client: %v, Recipe-Filename: %s", p.Addr, req.Filename)
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Rejecting all calls when this node is not the leader
	b, leaderAddr, err := loadshed.IsLeader(fs.Raft)
	if err != nil || !b && leaderAddr == "" {
		return nil, fmt.Errorf("not able")
	}
	if !b {
		return &pb.UploadResponse{Success: false, LeaderContainer: leaderAddr}, nil
	}

	if req == nil {
		return nil, fmt.Errorf("UploadResponse was nil")
	}

	//Priority Load Shedding
	if loadshed.PriorityShedding() {
		fs.logger.Info("Upload Request will be shed based on cpu threshold")
		return nil, fmt.Errorf("request been shedded based on our priority load shedding requirements")
	}

	//Probabilistic Load Shedding
	if loadshed.ProbabilisticShedding(true) {
		fs.logger.Info("Upload Request will be shed based on probabilistic")
		return nil, fmt.Errorf("request been shedded based on our probablistic load shedding requirements")
	}

	//Timeout based Load Shedding
	if loadshed.TimeoutShedding(ctx, fs.UploadQueue.EstimatedQueuingTime()) {
		fs.logger.Info("Upload Request will be shed based on timeout hints")
		return nil, fmt.Errorf("request been shedded based on our timeout load shedding requirements")
	}

	// Enqueuing
	job := &upload.Job{
		Ctx:    ctx,
		Req:    req,
		Result: make(chan upload.Res, 1), //Just a single result
	}

	if !fs.UploadQueue.Enqueue(job) {
		return nil, fmt.Errorf("upload queue: ressource exhausted, not enough memory left")
	}

	select {
	case res := <-job.Result:
		return res.Resp, res.Err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (fs *FileServer) DownloadRecipe(ctx context.Context, req *pb.RecipeDownloadRequest) (*pb.RecipeDownloadResponse, error) {
	p, ok := peer.FromContext(ctx)
	if ok {
		log.Printf("Received Download Request from client: %v, Recipe-Filename: %s", p.Addr, req.Filename)
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	//Load Shedding
	if loadshed.ProbabilisticShedding(false) {
		fs.logger.Info("Download Request will be shed based on probabilistic")
		return nil, fmt.Errorf("request been shedded based on our probablistic load shedding requirements")
	}

	if loadshed.TimeoutShedding(ctx, fs.DownloadQueue.EstimatedQueuingTime()) {
		fs.logger.Info("Download Request will be shed based on timeout hints")
		return nil, fmt.Errorf("request been shedded based on our timeout load shedding requirements")
	}

	log.Printf("Creating a job now for: %s", req.Filename)
	job := &download.Job{
		Ctx:    ctx,
		Req:    req,
		Result: make(chan download.Res, 1), //Just a single result
	}

	if !fs.DownloadQueue.Enqueue(job) {
		return nil, fmt.Errorf("downlaod queue: ressource exhausted, not enough memory left")
	}

	select {
	case res := <-job.Result:
		log.Printf("Received Download result from the Queue: %v (%v)", res.Resp, res.Err)
		return res.Resp, res.Err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// BackoffWithJitter calculates a random jittered backoff value
// Is public, so that remote functions can access it.
func BackoffWithJitter(backoff float64) float64 {
	backoff = backoff * 2
	return math.Min(rand.Float64()*backoff, 10000)
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
