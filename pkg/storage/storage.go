package storage

import (
	"context"
	"fmt"
	"go.etcd.io/bbolt"
	"go.uber.org/zap"
	"os"
	"time"
)

type Store struct {
	Database *bbolt.DB
	logger   *zap.SugaredLogger
	bucket   string
}

// NewStore constructs a new Store object based on a given Filepath to the Database file (Single .db File) with a predefined FileMode (0600)
func NewStore(path string, fileMode os.FileMode) *Store {
	db, err := bbolt.Open(path, fileMode, &bbolt.Options{Timeout: 1 * time.Second})
	logger := zap.S()
	if err != nil {

		logger.Fatalf("Opening the Database on Path: %s with FileMode: %v failed. Error: %v", path, fileMode, err)
		return nil
	}
	logger.Info("Successfully created Database with name", path)

	return &Store{
		Database: db,
		logger:   logger,
		bucket:   "recipes",
	}
}

// Recipe represents a Cooking Recipe which is stored in the Database and broadcasted through the system.
type Recipe struct {
	Filename string
	Content  string
}

func NewRecipe(filename string, content string) *Recipe {
	return &Recipe{
		Filename: filename,
		Content:  content,
	}
}

// StoreRecipe stores a given Recipe object in the underlying BoltDB database.
//
// Parameters:
//   - ctx: context used to support cancellation.
//   - recipe: pointer to the Recipe struct to store.
//
// Returns:
//   - error: nil if successful; otherwise, an error describing the failure.
func (s *Store) StoreRecipe(ctx context.Context, recipe *Recipe) error {
	return s.Database.Update(func(tx *bbolt.Tx) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		b, err := tx.CreateBucketIfNotExists([]byte(s.bucket))
		if err != nil {
			return err
		}

		// Store the recipe
		key := []byte(recipe.Filename)
		value := []byte(recipe.Content)

		s.logger.Infof("Storing with key: '%s' (length: %d)", recipe.Filename, len(key))
		s.logger.Infof("Storing with value length: %d", len(value))

		if err := b.Put(key, value); err != nil {
			s.logger.Errorf("Error storing recipe with key: %s, error: %v", recipe.Filename, err)
			return err
		}

		s.logger.Infof("Successfully stored recipe with key: '%s'", recipe.Filename)
		return nil
	})
}

// GetRecipe retrieves a Recipe from the BoltDB store using its UUID.
//
// Parameters:
//   - ctx: context used to support cancellation.
//   - id: UUID of the recipe to retrieve.
//
// Returns:
//   - *Recipe: pointer to the retrieved Recipe object, or nil if not found.
//   - error: nil if successful, or an error describing what went wrong.
func (s *Store) GetRecipe(ctx context.Context, filename string) (*Recipe, error) {
	s.logger.Infof("Starting to get recipe with UUID: %v", filename)
	var recipe Recipe
	err := s.Database.View(func(tx *bbolt.Tx) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		b := tx.Bucket([]byte(s.bucket))
		if b == nil {
			return fmt.Errorf("bucket: recipes not found")
		}
		s.logger.Infof("Fetched Bucket for name: %s, %v", filename, b.String())

		rawData := b.Get([]byte(filename))
		if rawData == nil {
			s.logger.Warnf("Got no rawData from the Database for Filename: %s", filename)
			return nil
		}

		data := string(rawData)
		recipe.Filename = filename
		recipe.Content = data

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("getting recipe from database failed with Error: %v", err)
	}

	return &recipe, err
}
