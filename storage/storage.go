package storage

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"go.etcd.io/bbolt"
	"log"
	"os"
	"time"
)

type Store struct {
	Database *bbolt.DB
}

// NewStore constructs a new Store object based on a given Filepath to the Database file (Single .db File) with a predefined FileMode (0600)
func NewStore(path string, fileMode os.FileMode) *Store {
	db, err := bbolt.Open(path, fileMode, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		log.Fatalf("Opening the Database on Path: %s with FileMode: %v failed. Error: %v", path, fileMode, err)
		return nil
	}
	return &Store{
		Database: db,
	}
}

// Recipe represents a Cooking Recipe which is stored in the Database and broadcasted through the system.
type Recipe struct {
	RecipeId uuid.UUID
	filename string
	content  string
	seen     []string
}

func NewRecipe(id uuid.UUID, filename string, content string) *Recipe {
	return &Recipe{
		RecipeId: id,
		filename: filename,
		content:  content,
	}
}

func (s *Store) StoreRecipe(ctx context.Context, recipe *Recipe) error {
	return s.Database.Update(func(tx *bbolt.Tx) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		b, err := tx.CreateBucketIfNotExists([]byte("recipes"))
		if err != nil {
			log.Fatalf("Error occured when Creating/Accessing the Bucket with Error: %v", err)
			return err
		}

		content, err := json.Marshal(recipe)
		if err != nil {
			log.Fatalf("Converting Recipe to JSON failed with Error: %v", err)
			return err
		}

		return b.Put(recipe.RecipeId[:], content)
	})
}
