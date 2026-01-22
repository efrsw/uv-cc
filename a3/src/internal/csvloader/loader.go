// Package csvloader allows to parse csv data from myanimelist
package csvloader

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
)

type Record struct {
	UID      string `json:"uid"`
	Profile  string `json:"profile"`
	AnimeUID string `json:"anime_uid"`
	Text     string `json:"text"`
	Score    string `json:"score"`
	Scores   string `json:"scores"`
	Link     string `json:"link"`
}

type InsertFunc func(key uint, value string) error

type ProgressFunc func(bytesRead int64)

type progressReader struct {
	reader     io.Reader
	bytesRead  int64
	onProgress ProgressFunc
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.bytesRead += int64(n)
	if pr.onProgress != nil {
		pr.onProgress(pr.bytesRead)
	}

	return n, err
}

type LoadResult struct {
	RecordsInserted int
	BytesRead       int64
	LimitReached    bool
}

func Load(filePath string, maxSizeMB int64, insertFn InsertFunc, progressFn ProgressFunc) (*LoadResult, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	maxSizeBytes := maxSizeMB * 1024 * 1024

	pr := &progressReader{
		reader:     file,
		onProgress: progressFn,
	}

	csvReader := csv.NewReader(pr)

	// create and configure csv reader
	header, err := csvReader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	colIndex := make(map[string]int)
	for i, col := range header {
		colIndex[col] = i
	}

	requiredCols := []string{"uid", "profile", "anime_uid", "text", "score", "scores", "link"}
	for _, col := range requiredCols {
		if _, ok := colIndex[col]; !ok {
			return nil, fmt.Errorf("missing required column: %s", col)
		}
	}

	// read csv until specified upload size reached
	rowCount := 0
	limitReached := false
	for {
		if pr.bytesRead >= maxSizeBytes {
			limitReached = true
			break
		}

		row, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read CSV row: %w", err)
		}

		rec := Record{
			UID:      row[colIndex["uid"]],
			Profile:  row[colIndex["profile"]],
			AnimeUID: row[colIndex["anime_uid"]],
			Text:     row[colIndex["text"]],
			Score:    row[colIndex["score"]],
			Scores:   row[colIndex["scores"]],
			Link:     row[colIndex["link"]],
		}

		key, err := strconv.ParseUint(rec.UID, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid uid '%s': %w", rec.UID, err)
		}

		valueBytes, err := json.Marshal(rec)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal record: %w", err)
		}

		// upload the row into the storage
		if err := insertFn(uint(key), string(valueBytes)); err != nil {
			return nil, fmt.Errorf("failed to insert key %d: %w", key, err)
		}

		rowCount++
	}

	return &LoadResult{
		RecordsInserted: rowCount,
		BytesRead:       pr.bytesRead,
		LimitReached:    limitReached,
	}, nil
}
