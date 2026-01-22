package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"univie.ac.at/efrsw/kvhash/bucket/handlers"
	"univie.ac.at/efrsw/kvhash/internal/directory"
	"univie.ac.at/efrsw/kvhash/internal/fs"
	"univie.ac.at/efrsw/kvhash/internal/logging"
	"univie.ac.at/efrsw/kvhash/internal/storage"
)

func main() {
	bucketIdx := flag.Int("idx", 0, "bucket index")
	basePort := flag.Int("baseport", directory.BucketPortBase, "starting port for a topology")
	dataDir := flag.String("datadir", ".", "directory for data, WAL, and log files")
	nodes := flag.Uint("nodes", directory.GetNumericEnv("NODE_COUNT", directory.DemoNodeCount), "Number of nodes")

	flag.Parse()

	// setup args
	bucketID := strconv.Itoa(*bucketIdx)
	procPort := *basePort + *bucketIdx
	dir := directory.NewDirectory(uint(*bucketIdx), uint(*basePort), uint(*nodes))

	// create chunk directory inside data directory
	chunkDir := filepath.Join(*dataDir, fmt.Sprintf("chunk_%d", *bucketIdx))
	if err := os.MkdirAll(chunkDir, 0o755); err != nil {
		slog.Error("failed to create chunk directory", "error", err)
		os.Exit(1)
	}

	// build file paths inside chunk directory
	dataFile := fmt.Sprintf("chunk_%d.json", *bucketIdx)
	walFile := fmt.Sprintf("chunk_%d.wal", *bucketIdx)
	logFile := fmt.Sprintf("bucket_%d.log", *bucketIdx)

	dataPath := filepath.Join(chunkDir, dataFile)
	walPath := filepath.Join(chunkDir, walFile)
	logPath := filepath.Join(chunkDir, logFile)

	f := fs.NewJSONFS(dataPath)

	// initialize logger
	logger, err := logging.NewBucketLogger(bucketID, logPath)
	if err != nil {
		slog.Error("failed to initialize logger", "error", err)
		os.Exit(1)
	}

	distributedStore, err := storage.NewDistributedStore(
		dir,
		f,
		walPath,
		dataFile,
		logger,
	)
	if err != nil {
		slog.Error("failed to initialize store", "error", err)
		os.Exit(1)
	}

	// log startup event
	logger.Info("startup", "data_file", dataFile, "wal_file", walFile)

	// setup server
	mux := http.NewServeMux()
	bh := handlers.NewBucketHandler(distributedStore, logger)
	bh.Register(mux)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", procPort),
		Handler: mux,
	}

	// listen for interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// start server
	go func() {
		slog.Info("starting server", "port", procPort, "bucket", *bucketIdx)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// block until shutdown signal is received then end gracefully
	sig := <-sigChan
	slog.Info("received shutdown signal", "signal", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}

	slog.Info("closing storage...")
	if err := distributedStore.Close(); err != nil {
		slog.Error("storage close error", "error", err)
		os.Exit(1)
	}

	slog.Info("shutdown complete")
}
