package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"univie.ac.at/efrsw/kvhash/internal/fs"
	"univie.ac.at/efrsw/kvhash/internal/storage"
)

type BucketClient struct {
	url *url.URL
}

func NewBucketClient(url *url.URL) *BucketClient {
	return &BucketClient{
		url: url,
	}
}

func (bc *BucketClient) Insert(k uint, v string) error {
	p := storage.NewProxyStore(bc.url)

	return p.Insert(k, v)
}

func (bc *BucketClient) Delete(k uint) error {
	p := storage.NewProxyStore(bc.url)

	return p.Delete(k)
}

func (bc *BucketClient) Search(k uint) (string, error) {
	p := storage.NewProxyStore(bc.url)
	return p.Search(k)
}

func (bc *BucketClient) Range(k1, k2 uint) ([]fs.KV, error) {
	params := url.Values{}
	params.Add("k1", strconv.FormatUint(uint64(k1), 10))
	params.Add("k2", strconv.FormatUint(uint64(k2), 10))

	finalURL := *bc.url
	finalURL.Path = "/values"
	finalURL.RawQuery = params.Encode()

	req, err := http.NewRequest(http.MethodGet, finalURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("client range: %w", err)
	}

	r, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("client range: %w", err)
	}
	defer r.Body.Close()

	if r.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("client range: %w", storage.ErrUnexpectedStatusCode)
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("client range: %w", err)
	}

	var values []fs.KV
	if err := json.Unmarshal(body, &values); err != nil {
		return nil, fmt.Errorf("client range: %w", err)
	}

	return values, nil
}
