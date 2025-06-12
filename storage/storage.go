package storage

import (
	"encoding/binary"
	"encoding/json"
	"github.com/hashicorp/memberlist"
	"go.etcd.io/bbolt"
	"log"
	"os"
	"time"
)

type Store struct {
	Database *bbolt.DB
}

type Recipe struct {
	RecipeId int
	filename string
	content  string
}

type Seen struct {
	RecipeId int
	nodes    []*memberlist.Node
}

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

func NewRecipe(filename string, content string) *Recipe {
	return &Recipe{
		filename: filename,
		content:  content,
	}
}

func (s *Store) StoreRecipe(recipe *Recipe) (uint64, error) {
	var id uint64
	return id, s.Database.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("recipes"))

		id, _ = b.NextSequence()
		recipe.RecipeId = int(id)

		content, err := json.Marshal(recipe)
		if err != nil {
			log.Fatalf("Converting Recipe to JSON failed with Error: %v", err)
			return err
		}

		return b.Put(itob(recipe.RecipeId), content)
	})
}

// itob returns an 8-byte big endian representation of the recipe's ID
func itob(id int) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(id))
	return b
}
