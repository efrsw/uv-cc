package storage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"univie.ac.at/efrsw/kvhash/internal/fs"
)

type ProxyStore struct {
	url *url.URL
}

var ErrUnexpectedStatusCode = errors.New("unexpected status code")

const HeaderFromProxy = "X-From-Proxy"

func NewProxyStore(u *url.URL) ProxyStore {
	return ProxyStore{url: u}
}

func (p *ProxyStore) Insert(k uint, v string) error {
	req := fs.KV{Key: k, Value: v}
	finalURL, err := url.JoinPath(p.url.String(), "values")
	if err != nil {
		return fmt.Errorf("proxy insert: %w", err)
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("proxy insert: %w", err)
	}

	r, err := http.Post(finalURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("proxy insert: %w", err)
	}
	defer r.Body.Close()

	if r.StatusCode == http.StatusOK {
		return nil
	} else {
		return fmt.Errorf("proxy insert: %w", ErrUnexpectedStatusCode)
	}
}

func (p *ProxyStore) Delete(k uint) error {
	finalURL, err := url.JoinPath(p.url.String(), "values", strconv.FormatUint(uint64(k), 10))
	if err != nil {
		return fmt.Errorf("proxy delete: %w", err)
	}

	req, err := http.NewRequest(http.MethodDelete, finalURL, nil)
	if err != nil {
		return fmt.Errorf("proxy delete: %w", err)
	}

	r, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("proxy delete: %w", err)
	}
	defer r.Body.Close()

	if r.StatusCode == http.StatusOK {
		return nil
	} else {
		return fmt.Errorf("proxy delete: %w", ErrUnexpectedStatusCode)
	}
}

func (p *ProxyStore) Search(k uint) (string, error) {
	finalURL, err := url.JoinPath(p.url.String(), "values", strconv.FormatUint(uint64(k), 10))
	if err != nil {
		return "", fmt.Errorf("proxy search: %w", err)
	}

	r, err := http.Get(finalURL)
	if err != nil {
		return "", fmt.Errorf("proxy search: %w", err)
	}
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return "", fmt.Errorf("proxy search: %w", err)
	}

	switch r.StatusCode {
	case http.StatusOK:
		return string(body), nil
	case http.StatusNotFound:
		return "", fmt.Errorf("proxy search: %w", ErrKeyNotFound)
	}

	return "", fmt.Errorf("proxy search: %w", ErrUnexpectedStatusCode)
}

func (p *ProxyStore) Range(k1, k2 uint, proxied bool) ([]fs.KV, error) {
	params := url.Values{}
	params.Add("k1", strconv.FormatUint(uint64(k1), 10))
	params.Add("k2", strconv.FormatUint(uint64(k2), 10))

	u := *p.url
	u.Path += "/values"
	u.RawQuery = params.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("proxy range: %w", err)
	}

	// add header so the processing node doesn't fan out requests further
	req.Header.Add(HeaderFromProxy, strconv.FormatBool(true))

	r, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("proxy range: %w", err)
	}
	defer r.Body.Close()

	if r.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("proxy range: %w", ErrUnexpectedStatusCode)
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("proxy range: %w", err)
	}

	var values []fs.KV
	if err := json.Unmarshal(body, &values); err != nil {
		return nil, fmt.Errorf("proxy range: %w", err)
	}

	return values, nil
}
