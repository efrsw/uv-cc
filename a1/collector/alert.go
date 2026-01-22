package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
)

type AlertRequest struct {
	Timestamp    string       `json:"timestamp"`
	Section      int          `json:"section"`
	Event        string       `json:"event"`
	KnownPersons []PersonName `json:"known-persons"`
	Image        *string      `json:"image,omitempty"`
	FrameUUID    *string      `json:"frame_uuid,omitempty"`
	ExtraInfo    *string      `json:"extra-info,omitempty"`
}

func PostAlertFromKnownPersons(req RecognizedPersonsRequest) error {
	alertReq := AlertRequest(req)

	body, err := json.Marshal(alertReq)
	if err != nil {
		return fmt.Errorf("error serializing alert request body: %w", err)
	}

	frameUUID := "none"
	if req.FrameUUID != nil {
		frameUUID = *req.FrameUUID
	}

	alertEndpoint := fmt.Sprintf(alertURL, alertHandle)

	config := RetryConfig{
		MaxRetries: 3,
		BaseDelay:  200 * time.Millisecond,
		FrameUUID:  frameUUID,
		Operation:  "alert post",
	}

	res, err := RetryableHTTPRequest(alertEndpoint, "application/json", body, config)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	log.Printf("[frame_uuid=%s] successfully alerted about %d known person(s) in section %d, event: %s",
		frameUUID, len(req.KnownPersons), req.Section, req.Event)
	return nil
}
