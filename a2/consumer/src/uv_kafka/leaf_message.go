package uvkafka

type LeafMessage struct {
	Id             int       `json:"id"`
	CameraId       int       `json:"camera_id"`
	Data           string    `json:"data"`
	GpsCoordinates []float64 `json:"gps_coordinates"`
	CreatedAt      LeafTime  `json:"created_at"`
	UpdatedAt      *LeafTime `json:"updated_at"`
	IsActive       bool      `json:"is_active"`
	IsDeleted      bool      `json:"is_deleted"`
	Disease        *string   `json:"disease"`
	Percentage     *float64  `json:"percentage"`
}

type LeafUpdateEvent struct {
	Topic     string
	Message   LeafMessage
	OnSuccess func()
}
