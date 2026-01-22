package uvkafka

import (
	"time"
)

type LeafTime struct {
	time.Time
}

func (lt *LeafTime) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}

	str := string(data)
	if len(str) >= 2 && str[0] == '"' && str[len(str)-1] == '"' {
		str = str[1 : len(str)-1]
	}

	t, err := time.Parse("2006-01-02T15:04:05.999999", str)
	if err != nil {
		return err
	}

	lt.Time = t
	return nil
}
