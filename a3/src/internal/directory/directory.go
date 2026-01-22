// Package directory provides information about system's topology
package directory

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
)

const (
	BucketPortBase = 47073 // my student id is 12347073
	DemoNodeCount  = 5
)

var ErrNodeIndexOutOfRange = errors.New("requested index is not present in topology")

type Directory struct {
	d         []*url.URL
	bucketIdx uint
}

func NewDirectory(bucketIdx, bucketBasePort, nodeCount uint) *Directory {
	d := make([]*url.URL, nodeCount)

	for i := range nodeCount {
		urlStr := fmt.Sprintf("http://localhost:%d/", bucketBasePort+i)
		u, _ := url.Parse(urlStr)
		d[i] = u
	}

	return &Directory{
		d:         d,
		bucketIdx: bucketIdx,
	}
}

func (d *Directory) GetBucketURL(i int) (*url.URL, error) {
	if i < 0 || i >= len(d.d) {
		return nil, ErrNodeIndexOutOfRange
	}

	return d.d[i], nil
}

func (d *Directory) GetTopologySize() int {
	return len(d.d)
}

func (d *Directory) GetBucketIdx() uint {
	return d.bucketIdx
}

func GetNumericEnv(key string, defaultValue uint) uint {
	if value := os.Getenv(key); value != "" {
		if v, err := strconv.ParseUint(value, 10, 64); err == nil {
			return uint(v)
		}
	}

	return defaultValue
}
