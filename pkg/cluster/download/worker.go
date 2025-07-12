package download

import (
	"ch3fs/pkg/cluster/loadshed"
	"ch3fs/pkg/storage"
	pb "ch3fs/proto"
	"context"
	lru "github.com/hashicorp/golang-lru"
	"go.uber.org/zap"
	"log"
	"time"
)

type Worker struct {
	Cache     *lru.ARCCache
	Store     *storage.Store
	logger    *zap.SugaredLogger
	Estimator *loadshed.Estimator
}

func NewWorker(cache *lru.ARCCache, store *storage.Store) *Worker {
	e := loadshed.NewEstimator(0.1)
	return &Worker{
		Cache:     cache,
		Store:     store,
		Estimator: e,
	}
}

func (w *Worker) ProcessDownloadRequest(ctx context.Context, req *pb.RecipeDownloadRequest) (*pb.RecipeDownloadResponse, error) {
	start := time.Now()
	if loadshed.ProbabilisticShedding(false) {
		w.logger.Infof("Download Request for Filename: %s, been shed based on Probabilistic", req.Filename)
		return nil, nil
	}

	if loadshed.TimeoutShedding(ctx, 200*time.Millisecond) {
		w.logger.Infof("Remaining time for execution is to low, request: %s, been shed", req.Filename)
		return nil, nil
	}

	cachedValueRaw, isInCache := w.Cache.Get(req.Filename)
	if isInCache {
		if recipe, succ := cachedValueRaw.(*storage.Recipe); succ {
			return &pb.RecipeDownloadResponse{
				Success:  true,
				Filename: recipe.Filename,
				Content:  []byte(recipe.Content),
			}, nil
		} else {
			log.Printf("Could not parse raw cache value to recipe")
		}
	}

	recipe, err := w.Store.GetRecipe(ctx, req.Filename)
	if err != nil {
		log.Printf("DownloadRecipe failed with %s", err)
		return nil, err
	}

	w.Cache.Add(recipe.Filename, recipe)
	
	w.Estimator.Update(time.Since(start))
	return &pb.RecipeDownloadResponse{
		Success:  true,
		Filename: recipe.Filename,
		Content:  []byte(recipe.Content),
	}, nil
}
