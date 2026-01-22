package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

type SectionRequest struct {
	Timestamp string       `json:"timestamp"`
	Section   int          `json:"section"`
	Event     string       `json:"event"`
	Persons   []PersonInfo `json:"persons"`
	Image     *string      `json:"image,omitempty"`
	FrameUUID *string      `json:"frame_uuid,omitempty"`
	ExtraInfo *string      `json:"extra-info,omitempty"`
}

func PostSection(req AnalyzedPersonsRequest) error {
	sectionReq := SectionRequest(req)

	body, err := json.Marshal(sectionReq)
	if err != nil {
		return fmt.Errorf("error serializing section request body: %w", err)
	}

	res, err := http.Post(fmt.Sprintf(sectionURL, sectionHandle), "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("error posting to section service: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode >= 200 && res.StatusCode < 300 {
		frameUUID := "none"
		if sectionReq.FrameUUID != nil {
			frameUUID = *sectionReq.FrameUUID
		}
		log.Printf("[frame_uuid=%s] successfully posted %d person(s) to section service for section %d, event: %s",
			frameUUID, len(sectionReq.Persons), sectionReq.Section, sectionReq.Event)
		return nil
	}

	return fmt.Errorf("unexpected status code from section service: %d", res.StatusCode)
}
