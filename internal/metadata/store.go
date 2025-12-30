package metadata

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v4"
)

type LastBackup struct {
	ShortId    string    `json:"shortId"`
	RemotePath string    `json:"path"`
	UploadedAt time.Time `json:"uploadedAt"`
	Size       int64     `json:"size"`
	Version    int       `json:"version"`
}

var (
	ErrNotFound = errors.New("last backup not found")
)

type Store interface {
	SaveLastBackup(provider string, backup LastBackup) error
	GetLastBackup(provider string) (*LastBackup, error)
	Close() error
}

type store struct {
	db *badger.DB
}

func NewStore(dataDir string) (Store, error) {
	opts := badger.DefaultOptions(dataDir).
		WithLoggingLevel(badger.WARNING)

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open badger: %w", err)
	}

	return &store{db: db}, nil
}

func (s *store) SaveLastBackup(provider string, backup LastBackup) error {
	key := []byte(fmt.Sprintf("last:%s", provider))

	data, err := json.Marshal(backup)
	if err != nil {
		return err
	}

	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, data)
	})
}
func (s *store) GetLastBackup(provider string) (*LastBackup, error) {
	key := []byte(fmt.Sprintf("last:%s", provider))

	var backup LastBackup

	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &backup)
		})
	})

	if errors.Is(err, badger.ErrKeyNotFound) {
		return nil, ErrNotFound
	}

	return &backup, err
}

func (s *store) Close() error {
	return s.db.Close()
}
