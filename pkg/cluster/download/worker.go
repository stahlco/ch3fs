package download

import (
	"ch3fs/pkg/cluster/loadshed"
	"ch3fs/pkg/storage"
	pb "ch3fs/proto"
	"context"
	lru "github.com/hashicorp/golang-lru"
	"log"
	"time"
)

type Worker struct {
	Cache     *lru.ARCCache
	Store     *storage.Store
	Estimator *loadshed.Estimator
}

// NewWorker creates and returns a new Worker instances.
//
// Parameters:
//
//   - cache: Pointer to an ARC cache for recipe storage
//
//   - store: Pointer to the underlying storage (bbolt)
//
//     Returns:
//
//   - *Worker
func NewWorker(cache *lru.ARCCache, store *storage.Store) *Worker {
	e := loadshed.NewEstimator(0.1)
	return &Worker{
		Cache:     cache,
		Store:     store,
		Estimator: e,
	}
}

// ProcessDownloadRequest handles a single download request.
// It applies load shedding as a second protection layer behind the queue to have a better control on the node stability.
// Then it will check the cache, and if a miss happens, it will check the storage.
// The processing time is tracked using a latency estimator, which is not working perfectly.
//
// Parameters:
//   - ctx: context (timeout) used to manage deadlines.
//   - req: pointer to the RecipeDownloadRequest message from the Job in the Queue
//
// Returns:
//   - *pb.RecipeDownloadResponse: the result of the download request
//   - error: if an error occurs during download or storage access
func (w *Worker) ProcessDownloadRequest(ctx context.Context, req *pb.RecipeDownloadRequest) (*pb.RecipeDownloadResponse, error) {
	start := time.Now()

	if loadshed.ProbabilisticShedding(false) {
		log.Printf("Download Request for Filename: %s, been shed based on Probabilistic", req.Filename)
		return nil, nil
	}

	if loadshed.TimeoutShedding(ctx, max(200*time.Millisecond, w.Estimator.Get())) {
		log.Printf("Remaining time for execution is to low, request: %s, been shed (%v)", req.Filename, w.Estimator.Get())
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

func (q *Queue) EstimatedQueuingTime() time.Duration {
	if q.worker == nil || q.worker.Estimator == nil || q.consumer == 0 {
		return 0
	}

	workerTime := q.worker.Estimator.Get()
	workers := q.consumer

	queueTime := float64(len(q.ch)) / float64(workers)
	return time.Duration(queueTime * float64(workerTime))
}
