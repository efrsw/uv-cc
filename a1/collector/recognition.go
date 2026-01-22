package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
)

type RecognitionRequest struct {
	Timestamp   string  `json:"timestamp"`
	Image       string  `json:"image"`
	Section     int     `json:"section"`
	Event       string  `json:"event"`
	Destination *string `json:"destination,omitempty"`
	FrameUUID   *string `json:"frame_uuid,omitempty"`
	ExtraInfo   *string `json:"extra-info,omitempty"`
}

type PersonName struct {
	Name string `json:"name"`
}

type RecognitionResponse struct {
	Timestamp    string       `json:"timestamp"`
	Section      int          `json:"section"`
	Event        string       `json:"event"`
	Image        string       `json:"image"`
	KnownPersons []PersonName `json:"known-persons"`
	FrameUUID    *string      `json:"frame_uuid"`
	ExtraInfo    *string      `json:"extra-info"`
}

func MakeRecognitionRequest(request FrameRequest) error {
	recognizeHandle := fmt.Sprintf(recognitionURL, recognitionHandle)
	dest := fmt.Sprintf(collectorURL, collectorAsyncRecognitionHandle)

	req := RecognitionRequest{
		Timestamp:   request.Timestamp,
		Image:       request.Image,
		Section:     request.Section,
		Event:       request.Event,
		Destination: &dest, // send results asynchronously to collector's handle
		FrameUUID:   &request.FrameUUID,
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
		Operation:  "recognition request",
	}

	r, err := RetryableHTTPRequest(recognizeHandle, "application/json", body, config)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	log.Printf("[frame_uuid=%s] recognition request sent, received status code %d",
		request.FrameUUID, r.StatusCode)
	return nil
}
