package storage

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"univie.ac.at/efrsw/kvhash/internal/directory"
	"univie.ac.at/efrsw/kvhash/internal/fs"
)

type DistributedStore struct {
	directory *directory.Directory
	fs        fs.FS

	// cache
	storage   map[uint]string
	storageMu sync.RWMutex

	// wal
	wal              *os.File
	walMu            sync.Mutex
	walPath          string
	logSize          int64
	compactThreshold int64

	// compaction
	compactTicker *time.Ticker
	stopChan      chan struct{}
	compactMu     sync.Mutex

	// logging
	logger   *slog.Logger
	dataFile string
}

func NewDistributedStore(d *directory.Directory, fs fs.FS, walPath string, dataFile string, logger *slog.Logger) (*DistributedStore, error) {
	store := &DistributedStore{
		directory:        d,
		fs:               fs,
		storage:          make(map[uint]string),
		walPath:          walPath,
		compactThreshold: 1024 * 1024, // 1MB
		compactTicker:    time.NewTicker(15 * time.Second),
		stopChan:         make(chan struct{}),
		logger:           logger,
		dataFile:         dataFile,
	}

	if err := store.initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize store: %w", err)
	}

	go store.backgroundCompactor()

	return store, nil
}

func (d *DistributedStore) Insert(k uint, v string) error {
	requiredIdx := hashUint(k) % uint(d.directory.GetTopologySize())

	// redirect insertion to another bucket if needed
	if requiredIdx != d.directory.GetBucketIdx() {
		u, err := d.directory.GetBucketURL(int(requiredIdx))
		if err != nil {
			return fmt.Errorf("distributed insert: %w", err)
		}

		d.logger.Info("proxy_insert", "key", k, "target_bucket", requiredIdx)
		p := NewProxyStore(u)
		return p.Insert(k, v)
	}

	// first write to WAL
	walEntry := fmt.Sprintf("INSERT:%d::%s\n", k, v)

	d.walMu.Lock()
	n, err := d.wal.WriteString(walEntry)
	if err != nil {
		d.walMu.Unlock()
		return fmt.Errorf("failed to write WAL: %w", err)
	}

	if err := d.wal.Sync(); err != nil {
		d.walMu.Unlock()
		return fmt.Errorf("failed to sync WAL: %w", err)
	}

	d.logSize += int64(n)
	shouldCompact := d.logSize > d.compactThreshold
	d.walMu.Unlock()

	// log the insert operation
	d.logger.Info("insert", "key", k, "file", filepath.Base(d.walPath))

	// update cache
	d.storageMu.Lock()
	d.storage[k] = v
	d.storageMu.Unlock()

	// run compaction if needed
	if shouldCompact {
		go func() {
			if err := d.compact(); err != nil {
				fmt.Printf("Warning: size-triggered compaction failed: %v\n", err)
			}
		}()
	}

	return nil
}

func (d *DistributedStore) Delete(k uint) error {
	requiredIdx := hashUint(k) % uint(d.directory.GetTopologySize())

	// redirect deletion to another bucket if needed
	if requiredIdx != d.directory.GetBucketIdx() {
		u, err := d.directory.GetBucketURL(int(requiredIdx))
		if err != nil {
			return fmt.Errorf("distributed delete: %w", err)
		}

		d.logger.Info("proxy_delete", "key", k, "target_bucket", requiredIdx)
		p := NewProxyStore(u)
		return p.Delete(k)
	}

	// first write to WAL
	walEntry := fmt.Sprintf("DELETE:%d\n", k)

	d.walMu.Lock()
	n, err := d.wal.WriteString(walEntry)
	if err != nil {
		d.walMu.Unlock()
		return fmt.Errorf("failed to write WAL: %w", err)
	}

	if err := d.wal.Sync(); err != nil {
		d.walMu.Unlock()
		return fmt.Errorf("failed to sync WAL: %w", err)
	}

	d.logSize += int64(n)
	shouldCompact := d.logSize > d.compactThreshold
	d.walMu.Unlock()

	// log the delete operation
	d.logger.Info("delete", "key", k, "file", filepath.Base(d.walPath))

	// update cache
	d.storageMu.Lock()
	delete(d.storage, k)
	d.storageMu.Unlock()

	// run compaction if needed
	if shouldCompact {
		go func() {
			if err := d.compact(); err != nil {
				fmt.Printf("Warning: size-triggered compaction failed: %v\n", err)
			}
		}()
	}

	return nil
}

func (d *DistributedStore) Search(k uint) (string, error) {
	requiredIdx := hashUint(k) % uint(d.directory.GetTopologySize())

	// redirect search to another bucket if needed
	if requiredIdx != d.directory.GetBucketIdx() {
		u, err := d.directory.GetBucketURL(int(requiredIdx))
		if err != nil {
			return "", fmt.Errorf("distributed search: %w", err)
		}

		d.logger.Info("proxy_search", "key", k, "target_bucket", requiredIdx)
		p := NewProxyStore(u)
		return p.Search(k)
	}

	// search cache
	d.storageMu.RLock()
	v, ok := d.storage[k]
	d.storageMu.RUnlock()

	// log the search operation
	d.logger.Info("search", "key", k, "found", ok)

	if !ok {
		return "", fmt.Errorf("distributed search: %w", ErrKeyNotFound)
	}

	return v, nil
}

func (d *DistributedStore) Range(k1, k2 uint, proxied bool) ([]fs.KV, error) {
	result := make([]fs.KV, 0)

	// if value comes from another node (i.e. proxied), we don't have to ask
	// other nodes for values
	if !proxied {
		d.logger.Info("range_start", "k1", k1, "k2", k2)

		for i := range d.directory.GetTopologySize() {
			if i == int(d.directory.GetBucketIdx()) {
				continue
			}

			u, err := d.directory.GetBucketURL(i)
			if err != nil {
				continue
			}

			d.logger.Info("range_query", "target_bucket", i)
			p := NewProxyStore(u)
			vals, err := p.Range(k1, k2, proxied)
			if err != nil {
				continue
			}

			d.logger.Info("range_response", "source_bucket", i, "result_count", len(vals))
			result = append(result, vals...)
		}
	}

	// search cache
	d.storageMu.RLock()
	for k, v := range d.storage {
		if k >= k1 && k <= k2 {
			result = append(result, fs.KV{Key: k, Value: v})
		}
	}
	d.storageMu.RUnlock()

	if !proxied {
		d.logger.Info("range_complete", "total_results", len(result))
	}

	return result, nil
}

func (d *DistributedStore) Close() error {
	// stop time-based compaction
	close(d.stopChan)
	d.compactTicker.Stop()

	// flush contents of wal
	if err := d.compact(); err != nil {
		return fmt.Errorf("final compaction failed: %w", err)
	}

	if d.wal != nil {
		return d.wal.Close()
	}

	return nil
}

func (d *DistributedStore) compact() error {
	d.compactMu.Lock()
	defer d.compactMu.Unlock()

	d.walMu.Lock()
	defer d.walMu.Unlock()

	if d.logSize == 0 {
		return nil
	}

	d.storageMu.RLock()
	snapshot := make(map[uint]string, len(d.storage))
	maps.Copy(snapshot, d.storage)
	keysCount := len(snapshot)
	d.storageMu.RUnlock()

	d.logger.Info("compaction_start", "file", filepath.Base(d.dataFile))

	// persist to json storage
	if err := d.fs.WriteStorage(snapshot); err != nil {
		return fmt.Errorf("compaction failed: %w", err)
	}

	// truncate wal
	if err := d.wal.Truncate(0); err != nil {
		return fmt.Errorf("failed to truncate WAL: %w", err)
	}

	if _, err := d.wal.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek WAL: %w", err)
	}

	d.logSize = 0

	d.logger.Info("compaction_complete", "file", filepath.Base(d.dataFile), "keys_written", keysCount)
	return nil
}

func (d *DistributedStore) backgroundCompactor() {
	for {
		select {
		case <-d.compactTicker.C:
			if err := d.compact(); err != nil {
				fmt.Printf("Warning: scheduled compaction failed: %v\n", err)
			}
		case <-d.stopChan:
			return
		}
	}
}

func (d *DistributedStore) initialize() error {
	snapshot, err := d.fs.LoadStorage()
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to load snapshot: %w", err)
	}

	d.storageMu.Lock()
	d.storage = snapshot
	if d.storage == nil {
		d.storage = make(map[uint]string)
	}
	d.storageMu.Unlock()

	wal, err := os.OpenFile(d.walPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open WAL: %w", err)
	}
	d.wal = wal

	if err := d.replayWAL(); err != nil {
		return fmt.Errorf("failed to replay WAL: %w", err)
	}

	return nil
}

func (d *DistributedStore) replayWAL() error {
	if _, err := d.wal.Seek(0, 0); err != nil {
		return err
	}

	scanner := bufio.NewScanner(d.wal)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		op, k, v, err := parseWALEntry(line)
		if err != nil {
			fmt.Printf("Warning: corrupt WAL entry: %s\n", line)
			continue
		}

		d.storageMu.Lock()
		switch op {
		case "INSERT":
			d.storage[k] = v
		case "DELETE":
			delete(d.storage, k)
		}
		d.storageMu.Unlock()

		d.logSize += int64(len(line)) + 1
	}

	if _, err := d.wal.Seek(0, 2); err != nil {
		return err
	}

	return scanner.Err()
}

func parseWALEntry(line string) (op string, key uint, value string, err error) {
	if len(line) < 8 {
		return "", 0, "", fmt.Errorf("invalid WAL entry")
	}

	switch line[:7] {
	case "INSERT:":
		parts := strings.SplitN(line, "::", 2)
		if len(parts) != 2 {
			return "", 0, "", fmt.Errorf("invalid INSERT entry: missing '::'")
		}

		var k uint64
		n, err := fmt.Sscanf(parts[0], "INSERT:%d", &k)
		if err != nil || n != 1 {
			return "", 0, "", fmt.Errorf("invalid INSERT entry: bad key")
		}

		return "INSERT", uint(k), parts[1], nil

	case "DELETE:":
		var k uint64
		n, err := fmt.Sscanf(line, "DELETE:%d", &k)
		if err != nil || n != 1 {
			return "", 0, "", fmt.Errorf("invalid DELETE entry")
		}
		return "DELETE", uint(k), "", nil
	}

	return "", 0, "", fmt.Errorf("unknown operation")
}

func hashUint(n uint) uint {
	// use fnv as it's stable across runs
	h := fnv.New64a()
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(n))
	h.Write(b)
	return uint(h.Sum64())
}
