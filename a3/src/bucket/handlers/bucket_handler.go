// Package handlers provide different parts of program API
package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"

	"univie.ac.at/efrsw/kvhash/internal/fs"
	"univie.ac.at/efrsw/kvhash/internal/storage"
)

type BucketHandler struct {
	kvStore storage.KvStore
	logger  *slog.Logger
}

func NewBucketHandler(s storage.KvStore, logger *slog.Logger) *BucketHandler {
	return &BucketHandler{
		kvStore: s,
		logger:  logger,
	}
}

func (bh *BucketHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /values", bh.HandleInsert)
	mux.HandleFunc("DELETE /values/{key}", bh.HandleDelete)
	mux.HandleFunc("GET /values/{key}", bh.HandleSearch)
	mux.HandleFunc("GET /values", bh.HandleRange)
}

func (bh *BucketHandler) HandleInsert(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var insertRequest fs.KV
	if err := json.Unmarshal(body, &insertRequest); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	err = bh.kvStore.Insert(insertRequest.Key, insertRequest.Value)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (bh *BucketHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	key, err := strconv.ParseUint(r.PathValue("key"), 10, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = bh.kvStore.Delete(uint(key))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (bh *BucketHandler) HandleSearch(w http.ResponseWriter, r *http.Request) {
	key, err := strconv.ParseUint(r.PathValue("key"), 10, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	v, err := bh.kvStore.Search(uint(key))
	if err != nil {
		if errors.Is(err, storage.ErrKeyNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Write([]byte(v))
}

func (bh *BucketHandler) HandleRange(w http.ResponseWriter, r *http.Request) {
	if !r.URL.Query().Has("k1") || !r.URL.Query().Has("k2") {
		http.Error(w, "missing k1 or k2 query parameters", http.StatusBadRequest)
		return
	}

	k1, err := strconv.ParseUint(r.URL.Query().Get("k1"), 10, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	k2, err := strconv.ParseUint(r.URL.Query().Get("k2"), 10, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	proxied, err := strconv.ParseBool(r.Header.Get(storage.HeaderFromProxy))
	if err != nil {
		// if request comes from client, we have to ask other nodes
		proxied = false
	}

	res, err := bh.kvStore.Range(uint(k1), uint(k2), proxied)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sort.Slice(res, func(i, j int) bool {
		return res[i].Key < res[j].Key
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(res)
}
