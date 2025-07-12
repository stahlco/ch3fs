package upload

import (
	"ch3fs/pkg/cluster/loadshed"
	"ch3fs/pkg/storage"
	pb "ch3fs/proto"
	"context"
	"fmt"
	"github.com/golang/protobuf/proto"
	lru "github.com/hashicorp/golang-lru"
	"github.com/hashicorp/raft"
	"log"
	"time"
)

type Worker struct {
	Cache     *lru.ARCCache
	Store     *storage.Store
	Raft      *raft.Raft
	Estimator *loadshed.Estimator
}

func NewWorker(cache *lru.ARCCache, store *storage.Store, raftNode *raft.Raft) *Worker {
	e := loadshed.NewEstimator(0.1)
	return &Worker{
		Cache:     cache,
		Store:     store,
		Raft:      raftNode,
		Estimator: e,
	}
}

func (w *Worker) ProcessUploadRequest(ctx context.Context, req *pb.RecipeUploadRequest) (*pb.UploadResponse, error) {
	start := time.Now()

	// Load Shedding
	if loadshed.ProbabilisticShedding(true) {
		log.Printf("Upload Request for Filename: %s, been shed based on Probabilistic", req.Filename)
		return nil, fmt.Errorf("upload been shed based on probabilistic (worker side)")
	}

	if loadshed.TimeoutShedding(ctx, 50*time.Millisecond) {
		log.Printf("Upload Request for Filename: %s, been shed based on Timeout Hints (%v)", req.Filename, w.Estimator.Get())
		return nil, fmt.Errorf("upload been shed based on timeout hint %v", w.Estimator.Get())
	}

	data, err := proto.Marshal(req)
	log.Printf("Marshalled Data: %s", data)
	if err != nil {
		return &pb.UploadResponse{Success: false}, fmt.Errorf("failed to marshal request: %w", err)
	}

	applyFuture := w.Raft.Apply(data, 5*time.Second)
	if err := applyFuture.Error(); err != nil {
		return &pb.UploadResponse{Success: false}, fmt.Errorf("raft apply failed: %w", err)
	}

	if resp, ok := applyFuture.Response().(*pb.UploadResponse); ok {
		w.Cache.Add(req.Filename, storage.Recipe{
			Filename: req.Filename,
			Content:  string(req.Content),
		})
		return resp, nil
	}

	w.Estimator.Update(time.Since(start))
	return &pb.UploadResponse{Success: false}, err
}
