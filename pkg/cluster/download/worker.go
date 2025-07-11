package download

import (
	"ch3fs/pkg/storage"
	pb "ch3fs/proto"
	"context"
	lru "github.com/hashicorp/golang-lru"
	"go.uber.org/zap"
	"log"
)

type Worker struct {
	Cache  *lru.ARCCache
	Store  *storage.Store
	logger *zap.SugaredLogger
}

func NewWorker(cache *lru.ARCCache, store *storage.Store) *Worker {
	return &Worker{
		Cache: cache,
		Store: store,
	}
}

func (w *Worker) ProcessDownloadRequest(ctx context.Context, req *pb.RecipeDownloadRequest) (*pb.RecipeDownloadResponse, error) {
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

	return &pb.RecipeDownloadResponse{
		Success:  true,
		Filename: recipe.Filename,
		Content:  []byte(recipe.Content),
	}, nil
}
