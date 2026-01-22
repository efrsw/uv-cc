// Package logging provides structured logging utilities for the distributed hash table
package logging

import (
	"fmt"
	"log/slog"
	"os"
)

func NewBucketLogger(bucketID string, logPath string) (*slog.Logger, error) {
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	handler := slog.NewTextHandler(logFile, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// only use info level, so no need for that field
			if a.Key == slog.LevelKey {
				return slog.Attr{}
			}

			// simplier timestamps
			if a.Key == slog.TimeKey {
				return a
			}
			return a
		},
	})

	logger := slog.New(handler).With("bucket_id", bucketID)
	return logger, nil
}
