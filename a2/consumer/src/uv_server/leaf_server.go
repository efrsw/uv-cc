package uvserver

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	uvmongo "univie.ac.at/efrsw/consumer/uv_mongo"
)

type LeafServer struct {
	Port  string
	Mongo *uvmongo.MongoConnection
}

type ImageResponse struct {
	Id             int       `json:"id"`
	CameraId       int       `json:"camera_id"`
	Data           string    `json:"data"`
	GpsCoordinates []float64 `json:"gps_coordinates"`
	CreatedAt      string    `json:"created_at"`
	UpdatedAt      *string   `json:"updated_at"`
	IsActive       bool      `json:"is_active"`
	IsDeleted      bool      `json:"is_deleted"`
	Disease        *string   `json:"disease"`
	Percentage     *float64  `json:"percentage"`
}

type TotalResponse struct {
	TotalImages int64 `json:"total_images"`
}

func (ls *LeafServer) ListenAndServe(ctx context.Context) error {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /image-plant/{plantType}/{id}", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		plantType := r.PathValue("plantType")
		plantId := r.PathValue("id")

		id, err := strconv.ParseInt(plantId, 10, 64)
		if err != nil {
			slog.Error("Error parsing plant id", "error", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		doc, err := ls.Mongo.GetPlantById(ctx, plantType, int(id))
		if err != nil {
			slog.Error("Failed to get image",
				"plantType", plantType,
				"imageId", id,
				"error", err)
			http.Error(w, "Image not found", http.StatusNotFound)
			return
		}

		response := ImageResponse{
			Id:             doc.Id,
			CameraId:       doc.CameraId,
			Data:           hex.EncodeToString(doc.Data.Data),
			GpsCoordinates: doc.GpsCoordinates,
			CreatedAt:      doc.CreatedAt.Format(time.RFC3339Nano),
			IsActive:       doc.IsActive,
			IsDeleted:      doc.IsDeleted,
			Disease:        doc.Disease,
			Percentage:     doc.Percentage,
		}

		if doc.UpdatedAt != nil {
			updatedStr := doc.UpdatedAt.Format(time.RFC3339Nano)
			response.UpdatedAt = &updatedStr
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			slog.Error("Failed to encode response", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		slog.Info("Retrieved image",
			"plantType", plantType,
			"imageId", id)
	})

	mux.HandleFunc("GET /image-plant/{plantType}/total", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		plantType := r.PathValue("plantType")

		total, err := ls.Mongo.GetTotalPlants(ctx, plantType)
		if err != nil {
			slog.Error("Failed to get total images",
				"plantType", plantType,
				"error", err)
			http.Error(w, "Failed to get total", http.StatusInternalServerError)
			return
		}

		response := TotalResponse{
			TotalImages: total,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			slog.Error("Failed to encode response", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		slog.Info("Retrieved total images",
			"plantType", plantType,
			"total", total)
	})

	mux.HandleFunc("GET /livenessProbe", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	})

	mux.HandleFunc("GET /readinessProbe", func(w http.ResponseWriter, r *http.Request) {
		if err := ls.Mongo.Ping(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	server := &http.Server{
		Addr:    ls.Port,
		Handler: mux,
	}

	// Graceful shutdown
	go func() {
		<-ctx.Done()
		slog.Info("Shutting down HTTP server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("HTTP server shutdown error", "error", err)
		}
	}()

	slog.Info("Starting HTTP server", "port", ls.Port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
