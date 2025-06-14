package storage

import (
	"context"
	"encoding/json"
	"fmt"
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
	Filename string
	Content  string
	Seen     []string
}

func NewRecipe(id uuid.UUID, filename string, content string, seen []string) *Recipe {
	return &Recipe{
		RecipeId: id,
		Filename: filename,
		Content:  content,
		Seen:     seen,
	}
}

// StoreRecipe saves the given Recipe into the BoltDB database.
// The Recipe is marshaled into JSON before being stored under its UUID key.
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

func (s *Store) UpdateRecipe(ctx context.Context, recipe *Recipe) error {
	return nil
}

// GetRecipe retrieves a Recipe from the BoltDB database using the provided UUID.
func (s *Store) GetRecipe(ctx context.Context, id uuid.UUID) (*Recipe, error) {
	var recipe Recipe

	err := s.Database.View(func(tx *bbolt.Tx) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		b := tx.Bucket([]byte("recipes"))
		if b == nil {
			return fmt.Errorf("bucket: recipes not found")
		}

		// id[:] converts the UUID into []byte
		rawData := b.Get(id[:])
		if rawData == nil {
			return nil
		}

		return json.Unmarshal(rawData, &recipe)
	})

	if err != nil {
		return nil, fmt.Errorf("getting recipe from database failed with Error: %v", err)
	}

	return &recipe, err
}

func (s *Store) RecipeExists(ctx context.Context, id uuid.UUID) (bool, error) {
	var exists bool

	err := s.Database.View(func(tx *bbolt.Tx) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		bucket := tx.Bucket([]byte("recipes"))
		if bucket == nil {
			return fmt.Errorf("bucket recipes not found")
		}

		// id[:] converts the UUID into []byte
		data := bucket.Get(id[:])
		exists = data != nil
		return nil
	})

	return exists, err
}
