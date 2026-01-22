package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
)

type FrameRequest struct {
	Timestamp string  `json:"timestamp"`
	Section   int     `json:"section"`
	Event     string  `json:"event"`
	Image     string  `json:"image"`
	FrameUUID string  `json:"frame_uuid"`
	ExtraInfo *string `json:"extra-info,omitempty"`
}

type AnalyzedPersonsRequest struct {
	Timestamp string       `json:"timestamp"`
	Section   int          `json:"section"`
	Event     string       `json:"event"`
	Persons   []PersonInfo `json:"persons"`
	Image     *string      `json:"image,omitempty"`
	FrameUUID *string      `json:"frame_uuid,omitempty"`
	ExtraInfo *string      `json:"extra-info,omitempty"`
}

type RecognizedPersonsRequest struct {
	Timestamp    string       `json:"timestamp"`
	Section      int          `json:"section"`
	Event        string       `json:"event"`
	KnownPersons []PersonName `json:"known-persons"`
	Image        *string      `json:"image,omitempty"`
	FrameUUID    *string      `json:"frame_uuid,omitempty"`
	ExtraInfo    *string      `json:"extra-info,omitempty"`
}

const (
	collectorURL                    = "http://collector-service:7070%s"
	recognitionURL                  = "http://recognition-service:7070%s"
	analyzeURL                      = "http://analysis-service:7070%s"
	alertURL                        = "http://alert-service:7070%s"
	sectionURL                      = "http://section-service:7070%s"
	sectionHandle                   = "/persons"
	collectorAsyncRecognitionHandle = "/known-persons"
	collectorAsyncAnalyzeHandle     = "/persons"
	recognitionHandle               = "/frame"
	analyticsHandle                 = "/frame"
	alertHandle                     = "/alerts"
)

func main() {
	log.Println("collector started")

	mux := http.NewServeMux()

	// Check liveness
	mux.HandleFunc("GET /livenessProbe", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Liveness check")
		w.WriteHeader(http.StatusOK)
	})

	// Check readiness
	mux.HandleFunc("GET /readinessProbe", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Readiness check")
		w.WriteHeader(http.StatusOK)
	})

	// Handle frame
	mux.HandleFunc("POST /frame", func(w http.ResponseWriter, r *http.Request) {
		log.Println("new frame received")

		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("error reading json body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var frameReq FrameRequest
		if err := json.Unmarshal(body, &frameReq); err != nil {
			log.Printf("error unmarshalling frame request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		log.Printf("[frame_uuid=%s] received new frame at %s, section: %d, event: %s",
			frameReq.FrameUUID, frameReq.Timestamp, frameReq.Section, frameReq.Event)

		// send recognition/analytics requests asynchronously
		go func() {
			err := MakeRecognitionRequest(frameReq)
			if err != nil {
				log.Printf("[frame_uuid=%s] error recognizing people: %v. dropping frame.", frameReq.FrameUUID, err)
			}
		}()

		go func() {
			err := MakeAnalyticsRequest(frameReq)
			if err != nil {
				log.Printf("[frame_uuid=%s] error analyzing people: %v. dropping frame.", frameReq.FrameUUID, err)
			}
		}()

		w.WriteHeader(http.StatusAccepted)
	})

	mux.HandleFunc("POST /persons", func(w http.ResponseWriter, r *http.Request) {
		log.Println("analyzed people received")

		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("error reading json body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var personsReq AnalyzedPersonsRequest
		if err := json.Unmarshal(body, &personsReq); err != nil {
			log.Printf("error reading request with analyzed people: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		frameUUID := "none"
		if personsReq.FrameUUID != nil {
			frameUUID = *personsReq.FrameUUID
		}
		log.Printf("[frame_uuid=%s] received persons data at %s, section: %d, event: %s, persons: %d",
			frameUUID, personsReq.Timestamp, personsReq.Section, personsReq.Event, len(personsReq.Persons))

		// post results to section service
		err = PostSection(personsReq)
		if err != nil {
			log.Printf("error posting to section service: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("POST /known-persons", func(w http.ResponseWriter, r *http.Request) {
		log.Println("recognized people received")

		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("error reading json body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var knownPersonsReq RecognizedPersonsRequest
		if err := json.Unmarshal(body, &knownPersonsReq); err != nil {
			log.Printf("error unmarshalling recognition service response: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		frameUUID := "none"
		if knownPersonsReq.FrameUUID != nil {
			frameUUID = *knownPersonsReq.FrameUUID
		}
		log.Printf("[frame_uuid=%s] received known persons data at %s, section: %d, event: %s, known persons: %d",
			frameUUID, knownPersonsReq.Timestamp, knownPersonsReq.Section, knownPersonsReq.Event, len(knownPersonsReq.KnownPersons))

		// post an alert
		err = PostAlertFromKnownPersons(knownPersonsReq)
		if err != nil {
			log.Printf("error posting alert: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})

	log.Fatal(http.ListenAndServe(":80", mux))
}
