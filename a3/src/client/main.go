package main

import (
	"bufio"
	"crypto/rand"
	"flag"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"univie.ac.at/efrsw/kvhash/internal/client"
	"univie.ac.at/efrsw/kvhash/internal/csvloader"
	"univie.ac.at/efrsw/kvhash/internal/directory"
)

type logEntry struct {
	timestamp time.Time
	line      string
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("expected 'insert', 'delete', 'search', 'range', 'merge', or 'upload' subcommands")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "insert":
		insertCmd := flag.NewFlagSet("insert", flag.ExitOnError)
		key := insertCmd.String("key", "", "Key to insert")
		value := insertCmd.String("value", "", "Value to insert")
		bp := insertCmd.Int("baseport", directory.BucketPortBase, "Buckets are listening starting from this port")
		nodeCount := insertCmd.Uint("nodes", directory.GetNumericEnv("NODE_COUNT", directory.DemoNodeCount), "Number of nodes")
		insertCmd.Parse(os.Args[2:])

		k, err := parseKey(*key)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}

		client, err := getRandomCoordinator(*bp, *nodeCount)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}

		if err := client.Insert(k, *value); err != nil {
			fmt.Fprintf(os.Stderr, "insert failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Insert successful")

	case "delete":
		deleteCmd := flag.NewFlagSet("delete", flag.ExitOnError)
		key := deleteCmd.String("key", "", "Key to delete")
		bp := deleteCmd.Int("baseport", directory.BucketPortBase, "Buckets are listening starting from this port")
		nodeCount := deleteCmd.Uint("nodes", directory.GetNumericEnv("NODE_COUNT", directory.DemoNodeCount), "Number of nodes")
		deleteCmd.Parse(os.Args[2:])

		k, err := parseKey(*key)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}

		client, err := getRandomCoordinator(*bp, *nodeCount)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}

		if err := client.Delete(k); err != nil {
			fmt.Fprintf(os.Stderr, "delete failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Delete successful")

	case "search":
		searchCmd := flag.NewFlagSet("search", flag.ExitOnError)
		key := searchCmd.String("key", "", "Key to search")
		bp := searchCmd.Int("baseport", directory.BucketPortBase, "Buckets are listening starting from this port")
		nodeCount := searchCmd.Uint("nodes", directory.GetNumericEnv("NODE_COUNT", directory.DemoNodeCount), "Number of nodes")
		searchCmd.Parse(os.Args[2:])

		k, err := parseKey(*key)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}

		client, err := getRandomCoordinator(*bp, *nodeCount)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}

		value, err := client.Search(k)
		if err != nil {
			fmt.Fprintf(os.Stderr, "search failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Value: %s\n", value)

	case "range":
		rangeCmd := flag.NewFlagSet("range", flag.ExitOnError)
		startKey := rangeCmd.String("start", "", "Start key")
		endKey := rangeCmd.String("end", "", "End key")
		bp := rangeCmd.Int("baseport", directory.BucketPortBase, "Buckets are listening starting from this port")
		nodeCount := rangeCmd.Uint("nodes", directory.GetNumericEnv("NODE_COUNT", directory.DemoNodeCount), "Number of nodes")
		rangeCmd.Parse(os.Args[2:])

		start, err := parseKey(*startKey)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid start key: %v\n", err)
			os.Exit(1)
		}

		end, err := parseKey(*endKey)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid end key: %v\n", err)
			os.Exit(1)
		}

		client, err := getRandomCoordinator(*bp, *nodeCount)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}

		results, err := client.Range(start, end)
		if err != nil {
			fmt.Fprintf(os.Stderr, "range query failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Results: %v\n", results)

	case "merge":
		mergeCmd := flag.NewFlagSet("merge", flag.ExitOnError)
		dir := mergeCmd.String("dir", "", "Directory to search for .log files")
		output := mergeCmd.String("output", "", "Output file path for merged logs")
		mergeCmd.Parse(os.Args[2:])

		if *dir == "" {
			fmt.Fprintf(os.Stderr, "directory is required (-dir)\n")
			os.Exit(1)
		}
		if *output == "" {
			fmt.Fprintf(os.Stderr, "output file is required (-output)\n")
			os.Exit(1)
		}

		entries, err := collectLogEntries(*dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to collect log entries: %v\n", err)
			os.Exit(1)
		}

		sort.Slice(entries, func(i, j int) bool {
			return entries[i].timestamp.Before(entries[j].timestamp)
		})

		if err := writeMergedLogs(*output, entries); err != nil {
			fmt.Fprintf(os.Stderr, "failed to write merged logs: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Merged %d log entries into %s\n", len(entries), *output)

	case "upload":
		uploadCmd := flag.NewFlagSet("upload", flag.ExitOnError)
		filePath := uploadCmd.String("file", "", "CSV file to upload")
		maxSize := uploadCmd.Int64("max", 200, "Maximum file size in megabytes")
		bp := uploadCmd.Int("baseport", directory.BucketPortBase, "Buckets are listening starting from this port")
		nodeCount := uploadCmd.Uint("nodes", directory.GetNumericEnv("NODE_COUNT", directory.DemoNodeCount), "Number of nodes")
		uploadCmd.Parse(os.Args[2:])

		if *filePath == "" {
			fmt.Fprintf(os.Stderr, "file is required (-file)\n")
			os.Exit(1)
		}

		// select coordinator on each insert
		insertFunc := func(k uint, v string) error {
			bucketClient, err := getRandomCoordinator(*bp, *nodeCount)
			if err != nil {
				return fmt.Errorf("failed to get coordinator: %w", err)
			}

			return bucketClient.Insert(k, v)
		}

		result, err := csvloader.Load(
			*filePath,
			*maxSize,
			insertFunc,
			func(bytesRead int64) {
				fmt.Printf("\rUploaded: %.2f MB", float64(bytesRead)/(1024*1024))
			},
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nupload failed: %v\n", err)
			os.Exit(1)
		}

		if result.LimitReached {
			fmt.Printf("\rUploaded: %.2f MB - %d records inserted (limit reached)\n",
				float64(result.BytesRead)/(1024*1024), result.RecordsInserted)
		} else {
			fmt.Printf("\rUploaded: %.2f MB - %d records inserted\n",
				float64(result.BytesRead)/(1024*1024), result.RecordsInserted)
		}

	default:
		fmt.Println("unknown subcommand")
		os.Exit(1)
	}
}

func getRandomCoordinator(bp int, nodeCount uint) (*client.BucketClient, error) {
	r, err := rand.Int(rand.Reader, big.NewInt(int64(nodeCount)))
	if err != nil {
		return nil, fmt.Errorf("failed to generate random number: %w", err)
	}

	u, err := url.Parse(fmt.Sprintf("http://localhost:%d", bp+int(r.Int64())))
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	return client.NewBucketClient(u), nil
}

func parseKey(key string) (uint, error) {
	k, err := strconv.ParseUint(key, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid key: %w", err)
	}

	return uint(k), nil
}

func collectLogEntries(dir string) ([]logEntry, error) {
	var entries []logEntry

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if !strings.HasSuffix(path, ".log") {
			return nil
		}

		fileEntries, err := parseLogFile(path)
		if err != nil {
			return fmt.Errorf("failed to parse %s: %w", path, err)
		}
		entries = append(entries, fileEntries...)

		return nil
	})

	if err != nil {
		return nil, err
	}

	return entries, nil
}

func parseLogFile(path string) ([]logEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entries []logEntry
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		ts, err := extractTimestamp(line)
		if err != nil {
			continue
		}

		entries = append(entries, logEntry{
			timestamp: ts,
			line:      line,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return entries, nil
}

func extractTimestamp(line string) (time.Time, error) {
	if !strings.HasPrefix(line, "time=") {
		return time.Time{}, fmt.Errorf("no timestamp found")
	}

	end := strings.Index(line, " ")
	if end == -1 {
		end = len(line)
	}

	tsStr := line[5:end]
	ts, err := time.Parse(time.RFC3339Nano, tsStr)
	if err != nil {
		ts, err = time.Parse(time.RFC3339, tsStr)
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to parse timestamp: %w", err)
		}
	}

	return ts, nil
}

func writeMergedLogs(path string, entries []logEntry) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, entry := range entries {
		if _, err := writer.WriteString(entry.line + "\n"); err != nil {
			return err
		}
	}

	return writer.Flush()
}
