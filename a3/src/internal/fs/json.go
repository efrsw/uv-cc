// Package fs provides primitives to work with persistence layer
package fs

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

type FS interface {
	LoadStorage() (map[uint]string, error)
	WriteStorage(values map[uint]string) error
}

type JSONFS struct {
	filepath string
}

type KV struct {
	Key   uint   `json:"key"`
	Value string `json:"value"`
}

func NewJSONFS(path string) *JSONFS {
	return &JSONFS{
		filepath: path,
	}
}

func (j *JSONFS) LoadStorage() (map[uint]string, error) {
	var storage []KV
	err := loadJSON(j.filepath, &storage)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[uint]string), nil
		}

		if errors.Is(err, io.EOF) {
			return make(map[uint]string), nil
		}

		return nil, fmt.Errorf("loading storage: %w", err)
	}

	storageMap := make(map[uint]string, len(storage))
	for _, v := range storage {
		storageMap[v.Key] = v.Value
	}

	return storageMap, nil
}

// WriteStorage atomically (with the help of temp file and mv) flushes provided map on disk.
func (j *JSONFS) WriteStorage(values map[uint]string) error {
	storage := make([]KV, 0, len(values))
	for k, v := range values {
		storage = append(storage, KV{k, v})
	}

	tempPath := j.filepath + ".tmp"
	tempFile, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}

	encoder := json.NewEncoder(tempFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(storage); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
		return fmt.Errorf("encoding to temp file: %w", err)
	}

	if err := tempFile.Sync(); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
		return fmt.Errorf("syncing temp file: %w", err)
	}

	_ = tempFile.Close()

	if err := os.Rename(tempPath, j.filepath); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}

func loadJSON(filename string, target any) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	return decoder.Decode(target)
}
