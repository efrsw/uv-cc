package uvmongo

import (
	"encoding/hex"
	"log/slog"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	uvkafka "univie.ac.at/efrsw/consumer/uv_kafka"
)

type LeafDocument struct {
	Id             int         `bson:"id"`
	CameraId       int         `bson:"camera_id"`
	Data           bson.Binary `bson:"data"`
	GpsCoordinates []float64   `bson:"gps_coordinates"`
	CreatedAt      time.Time   `bson:"created_at"`
	UpdatedAt      *time.Time  `bson:"updated_at"`
	IsActive       bool        `bson:"is_active"`
	IsDeleted      bool        `bson:"is_deleted"`
	Disease        *string     `bson:"disease"`
	Percentage     *float64    `bson:"percentage"`
}

func (p *LeafDocument) FromMessage(m uvkafka.LeafMessage) {
	p.Id = m.Id
	p.CameraId = m.CameraId
	p.GpsCoordinates = m.GpsCoordinates
	p.CreatedAt = m.CreatedAt.Time
	p.IsActive = m.IsActive
	p.IsDeleted = m.IsDeleted
	p.Disease = m.Disease
	p.Percentage = m.Percentage

	if m.UpdatedAt != nil {
		t := m.UpdatedAt.Time
		p.UpdatedAt = &t
	}

	// Clean hex string - remove whitespace
	cleanData := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
			return -1
		}
		return r
	}, m.Data)

	// Decode hex string to bytes
	decoded, err := hex.DecodeString(cleanData)
	if err != nil {
		slog.Error("Failed to decode hex image data",
			"messageId", m.Id,
			"dataLength", len(m.Data),
			"cleanedLength", len(cleanData),
			"error", err)
		p.Data = bson.Binary{Data: []byte{}, Subtype: 0x00}
	} else {
		p.Data = bson.Binary{Data: decoded, Subtype: 0x00}
	}
}
