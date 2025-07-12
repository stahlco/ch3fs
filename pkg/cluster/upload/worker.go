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
	"go.uber.org/zap"
	"log"
	"time"
)

type Worker struct {
	Cache     *lru.ARCCache
	Store     *storage.Store
	Raft      *raft.Raft
	logger    *zap.SugaredLogger
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
