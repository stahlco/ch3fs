package transport

import (
	"ch3fs/pkg/cluster/download"
	"ch3fs/pkg/cluster/loadshed"
	"ch3fs/pkg/cluster/upload"
	pb "ch3fs/proto"
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"time"

	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/raft"
	"go.uber.org/zap"
	"google.golang.org/grpc/peer"
)

const GRPCPort = ":8080"

var (
	requestCycle = 0
	lastScaleUp  time.Time
)

type FileServer struct {
	pb.FileSystemServer
	UploadQueue       *upload.Queue
	DownloadQueues    *[]download.Queue
	DownloadEstimator *loadshed.Estimator
	logger            *zap.SugaredLogger
	Raft              *raft.Raft
}

// NewFileServer initializes a new FileServer with upload/download queues,
// a Raft instance for consensus, and a logger for structured logging.
func NewFileServer(uq *upload.Queue, dq *[]download.Queue, raft *raft.Raft, logger *zap.SugaredLogger) *FileServer {
	return &FileServer{
		UploadQueue:       uq,
		DownloadQueues:    dq,
		DownloadEstimator: loadshed.NewEstimator(0.1),
		Raft:              raft,
		logger:            logger,
	}
}

// UploadRecipe handles incoming requests to upload a recipe file.
// It checks for leadership status using Raft and enforces load shedding
// strategies such as priority-based, probabilistic, and timeout-based shedding.
//
// If the request is accepted, it is enqueued into the UploadQueue for processing.
// The method blocks until a result is returned or the context is cancelled.
func (fs *FileServer) UploadRecipe(ctx context.Context, req *pb.RecipeUploadRequest) (*pb.UploadResponse, error) {
	p, ok := peer.FromContext(ctx)
	if ok {
		log.Printf("Received Upload Request from client: %v, Recipe-Filename: %s", p.Addr, req.Filename)
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Rejecting all calls when this node is not the leader
	isLeader, leaderAddr, err := loadshed.IsLeader(fs.Raft)
	if err != nil || !isLeader && leaderAddr == "" {
		return nil, fmt.Errorf("not able")
	}
	if !isLeader {
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
	if loadshed.TimeoutShedding(ctx, 5*time.Second) {
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

// DownloadRecipe handles gRPC requests to download a recipe file.
// It applies probabilistic and timeout-based load shedding before
// enqueueing the job in the DownloadQueues.
//
// The method waits for a response from the queue or returns an error
// if the context is cancelled or the system is overloaded.
func (fs *FileServer) DownloadRecipe(ctx context.Context, req *pb.RecipeDownloadRequest) (*pb.RecipeDownloadResponse, error) {
	p, ok := peer.FromContext(ctx)
	if ok {
		log.Printf("Received Download Request from client: %v, Recipe-Filename: %s", p.Addr, req.Filename)
	}

	start := time.Now()

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	//Load Shedding
	if loadshed.ProbabilisticShedding(false) {
		fs.logger.Info("Download Shed: Probabilistic")
		return nil, fmt.Errorf("request shed probablistically")
	}

	timeout := max(10*time.Second, fs.DownloadEstimator.Get())
	if loadshed.TimeoutShedding(ctx, timeout) {
		log.Printf("Download shed: timeout treshold exceeded, %v", timeout)
		return nil, fmt.Errorf("request shed due to timeout")
	}

	job := &download.Job{
		Ctx:    ctx,
		Req:    req,
		Result: make(chan download.Res, 1), //Just a single result
	}

	if requestCycle%25 == 0 && !fs.ScaleDownloadQueuesUp() {
		fs.ScaleDownloadQueuesDown()
	}
	requestCycle++

	queue := fs.LoadBasedEnqueueing()
	if queue == nil || !queue.Enqueue(job) {
		return nil, fmt.Errorf("downlaod queue: ressource exhausted, not enough memory left")
	}

	select {
	case res := <-job.Result:
		log.Printf("Received Download result from the Queue: %v (%v)", res.Resp, res.Err)
		fs.DownloadEstimator.Update(time.Since(start))
		return res.Resp, res.Err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (fs *FileServer) LoadBasedEnqueueing() *download.Queue {
	var (
		smallestQueue *download.Queue
		minLoadRatio  = 1.1
	)

	for i := range *fs.DownloadQueues {
		queue := &(*fs.DownloadQueues)[i]

		if queue.Cap == 0 {
			continue
		}

		loadRatio := float64(queue.Len()) / float64(queue.Cap)
		if loadRatio < minLoadRatio {
			minLoadRatio = loadRatio
			smallestQueue = queue
		}
	}

	return smallestQueue
}

func (fs *FileServer) ScaleDownloadQueuesDown() bool {
	queues := *fs.DownloadQueues

	if len(queues) <= 2 {
		return false
	}

	for i := 0; i < len(queues); i++ {
		if queues[i].Len() == 0 || time.Since(lastScaleUp) > 10*time.Second {
			log.Printf("Removing idle queue at index %d", i)
			*fs.DownloadQueues = append(queues[:i], queues[i+1:]...)
			return true
		}
	}

	return false
}

func (fs *FileServer) ScaleDownloadQueuesUp() bool {
	maximumCap := 0
	usedCap := 0
	var w *download.Worker

	// if queues total 85% used -> create new with n workers
	for _, q := range *fs.DownloadQueues {
		maximumCap += q.Cap
		usedCap += q.Len()
		if w == nil {
			w = q.Worker
		}
	}

	log.Printf("Current used Cap: %d / %d", usedCap, maximumCap)
	curr := float64(usedCap) / float64(maximumCap)
	log.Printf("Current Queue Usage: %f", curr)

	if curr >= 0.65 {
		log.Printf("Creating a new Queue, current queues: %d", len(*fs.DownloadQueues))
		newQueue := download.NewDownloadQueue(100, w)
		*fs.DownloadQueues = append(*fs.DownloadQueues, *newQueue)
		fs.logger.Infof("Created new DownloadQueue")
		log.Printf("Created a new Queue, current queues: %d", len(*fs.DownloadQueues))
		lastScaleUp = time.Now()
		return true
	}

	return false
}

// BackoffWithJitter calculates a random jittered backoff value
// Is public, so that remote functions can access it.
func BackoffWithJitter(backoff float64) float64 {
	backoff = backoff * 2
	return math.Min(rand.Float64()*backoff, 10000)
}

// ListContainsContainerName checks whether the given container name
// exists in the current memberlist cluster.
//
// Returns true if the container name is present, otherwise false.
func ListContainsContainerName(l *memberlist.Memberlist, container string) bool {
	for _, m := range l.Members() {
		if m.Name == container {
			return true
		}
	}
	return false
}
