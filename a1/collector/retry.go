package main

import (
	"bytes"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"
)

type RetryConfig struct {
	MaxRetries int
	BaseDelay  time.Duration
	FrameUUID  string
	Operation  string
}

// retryable http request with exponential backoff + jitter
func RetryableHTTPRequest(url string, contentType string, bodyBytes []byte, config RetryConfig) (*http.Response, error) {
	var lastErr error

	for attempt := 1; attempt <= config.MaxRetries; attempt++ {
		resp, err := http.Post(url, contentType, bytes.NewBuffer(bodyBytes))

		if err != nil {
			lastErr = err
			if attempt < config.MaxRetries {
				delay := calculateDelayWithJitter(config.BaseDelay, attempt)
				log.Printf("[frame_uuid=%s] %s failed (attempt %d/%d), retrying in %v: %v",
					config.FrameUUID, config.Operation, attempt, config.MaxRetries, delay, err)
				time.Sleep(delay)
				continue
			}

			return nil, fmt.Errorf("%s failed after %d attempts: %w",
				config.Operation, config.MaxRetries, err)
		}

		// success (2xx status codes)
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		}

		// non-retryable client errors (4xx)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			resp.Body.Close()
			return nil, fmt.Errorf("%s failed with client error: status %d",
				config.Operation, resp.StatusCode)
		}

		// retryable server errors (5xx)
		resp.Body.Close()
		lastErr = fmt.Errorf("status %d", resp.StatusCode)
		if attempt < config.MaxRetries {
			delay := calculateDelayWithJitter(config.BaseDelay, attempt)
			log.Printf("[frame_uuid=%s] %s failed with status %d (attempt %d/%d), retrying in %v",
				config.FrameUUID, config.Operation, resp.StatusCode, attempt, config.MaxRetries, delay)
			time.Sleep(delay)
			continue
		}
	}

	return nil, fmt.Errorf("%s failed after %d attempts: %w", config.Operation, config.MaxRetries, lastErr)
}

func calculateDelayWithJitter(baseDelay time.Duration, attempt int) time.Duration {
	delay := baseDelay * time.Duration(1<<(attempt-1))     // exp backoff
	jitter := time.Duration(rand.Int63n(int64(delay / 2))) // jitter

	return delay + jitter
}
