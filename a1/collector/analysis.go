package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
)

type PersonInfo struct {
	Age    string `json:"age"`
	Gender string `json:"gender"`
}

type AnalyzeRequest struct {
	Timestamp   string  `json:"timestamp"`
	Image       string  `json:"image"`
	Section     int     `json:"section"`
	Event       string  `json:"event"`
	Destination *string `json:"destination,omitempty"`
	FrameUUID   string  `json:"frame_uuid"`
	ExtraInfo   *string `json:"extra-info,omitempty"`
}

type AnalyzeResponse struct {
	Timestamp string       `json:"timestamp"`
	Section   int          `json:"section"`
	Event     string       `json:"event"`
	Persons   []PersonInfo `json:"persons"`
	ExtraInfo string       `json:"extra-info"`
	FrameUUID string       `json:"frame_uuid"`
	Image     string       `json:"image"`
}

func MakeAnalyticsRequest(request FrameRequest) error {
	analyzeHandle := fmt.Sprintf(analyzeURL, analyticsHandle)
	dest := fmt.Sprintf(collectorURL, collectorAsyncAnalyzeHandle)

	req := AnalyzeRequest{
		Timestamp:   request.Timestamp,
		Image:       request.Image,
		Section:     request.Section,
		Event:       request.Event,
		Destination: &dest, // send results asynchronously to collector's handle
		FrameUUID:   request.FrameUUID,
		ExtraInfo:   request.ExtraInfo,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	config := RetryConfig{
		MaxRetries: 3,
		BaseDelay:  200 * time.Millisecond,
		FrameUUID:  request.FrameUUID,
		Operation:  "analytics request",
	}

	r, err := RetryableHTTPRequest(analyzeHandle, "application/json", body, config)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	log.Printf("[frame_uuid=%s] analytics request sent, received status code %d",
		request.FrameUUID, r.StatusCode)
	return nil
}
