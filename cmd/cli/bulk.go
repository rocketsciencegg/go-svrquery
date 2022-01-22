package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/multiplay/go-svrquery/lib/svrquery"
	"github.com/multiplay/go-svrquery/lib/svrquery/protocol"
)

const (
	numWorkers = 100
)

var (
	errNoItem       = errors.New("no item")
	errEntryInvalid = errors.New("invalid entry")
)

// BulkResponseItem contains the information about the query being performed
// against a single server.
type BulkResponseItem struct {
	Address    string                      `json:"address,omitempty"`
	ServerInfo *BulkResponseServerInfoItem `json:"serverInfo,omitempty"`
	Error      string                      `json:"error,omitempty"`
}

// encode writes the JSON encoded version of i to w using the encoder e which writes to b.
// It strips the trailing \n from the output before writing to w.
func (i *BulkResponseItem) encode(w io.Writer, b *bytes.Buffer, e *json.Encoder) error {
	defer b.Reset()

	if err := e.Encode(i); err != nil {
		return fmt.Errorf("encode item %v: %w", i, err)
	}

	if _, err := w.Write(bytes.TrimRight(b.Bytes(), "\n")); err != nil {
		return fmt.Errorf("write item: %w", err)
	}

	return nil
}

// BulkResponseServerInfoItem containing basic server information.
type BulkResponseServerInfoItem struct {
	CurrentPlayers int64  `json:"currentPlayers"`
	MaxPlayers     int64  `json:"maxPlayers"`
	Map            string `json:"map"`
}

// queryBulk queries a bulk set of servers from a query file writing the JSON results to output.
func queryBulk(file string, output io.Writer) (err error) {
	work := make(chan string, numWorkers)              // Buffered to ensure we can busy all workers.
	results := make(chan BulkResponseItem, numWorkers) // Buffered to improve worker concurrency.

	// Create a pool of workers to process work.
	var wgWorkers sync.WaitGroup
	wgWorkers.Add(numWorkers)
	for w := 1; w <= numWorkers; w++ {
		c, err := svrquery.NewBulkClient()
		if err != nil {
			close(work) // Ensure that existing workers return.
			return fmt.Errorf("bulk client: %w", err)
		}

		go func() {
			defer wgWorkers.Done()
			worker(work, results, c)
		}()
	}

	// Create a writer to write the results to output as they become available.
	errc := make(chan error)
	go func() {
		errc <- writer(output, results)
	}()

	// Queue work onto the channel.
	if err = producer(file, work); err != nil {
		err = fmt.Errorf("producer: %w", err)
	}

	// Wait for all workers to complete so that we can safely close results
	// that will trigger writer to return once its processed all results.
	wgWorkers.Wait()
	close(results)

	if werr := <-errc; werr != nil {
		if err != nil {
			return fmt.Errorf("%w, writer: %s", err, werr)
		}
		return fmt.Errorf("writer: %w", werr)
	}

	return err
}

// writer writes results as JSON encoded array to w.
func writer(w io.Writer, results <-chan BulkResponseItem) (err error) {
	if _, err = w.Write([]byte{'['}); err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	// Do our best to write valid JSON by ensuring we always write
	// the closing ]. If a previous encode fails, this could still
	// be insufficient.
	defer func() {
		if _, werr := w.Write([]byte("]\n")); werr != nil {
			werr = fmt.Errorf("write trailer: %w", err)
			if err == nil {
				err = werr
			}
		}
	}()

	var b bytes.Buffer
	e := json.NewEncoder(&b)

	// Process the first item before looping so separating
	// comma can be written easily.
	i, ok := <-results
	if !ok {
		return nil
	}

	if err := i.encode(w, &b, e); err != nil {
		return err
	}

	for i := range results {
		if _, err := w.Write([]byte(",")); err != nil {
			return fmt.Errorf("write set: %w", err)
		}

		if err := i.encode(w, &b, e); err != nil {
			return err
		}
	}

	return nil
}

// producer reads lines from file sending them to work.
// work will be closed before return.
func producer(file string, work chan<- string) error {
	defer close(work)

	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		work <- s.Text()
	}

	return s.Err()
}

// worker calls processBulkEntry for each item read from work, writing the result to results.
func worker(work <-chan string, results chan<- BulkResponseItem, client *svrquery.BulkClient) {
	for e := range work {
		results <- processBulkEntry(e, client)
	}
}

// processBulkEntry decodes and processes an entry returning an item containing the result or error.
func processBulkEntry(entry string, client *svrquery.BulkClient) (item BulkResponseItem) {
	querySection, addressSection, err := parseEntry(entry)
	if err != nil {
		item.Error = fmt.Sprintf("parse file entry: %s", err)
		return item
	}

	item.Address = addressSection

	// If the query contains any options retrieve and use them.
	querySection, options, err := parseOptions(querySection)
	if err != nil {
		item.Error = err.Error()
		return item
	}

	resp, err := client.Query(querySection, addressSection, options...)
	if err != nil {
		item.Error = fmt.Sprintf("query client: %s", err)
		return item
	}

	item.ServerInfo = &BulkResponseServerInfoItem{
		CurrentPlayers: resp.NumClients(),
		MaxPlayers:     resp.MaxClients(),
		Map:            "UNKNOWN",
	}

	if currentMap, ok := resp.(protocol.Mapper); ok {
		item.ServerInfo.Map = currentMap.Map()
	}
	return item
}

// pareEntry parses the details from entry returning the query and address sections.
func parseEntry(entry string) (querySection, addressSection string, err error) {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return "", "", fmt.Errorf("parse entry %q: %w", entry, errNoItem)
	}

	sections := strings.Split(entry, " ")
	if len(sections) != 2 {
		return "", "", fmt.Errorf("%w %q: wrong number of sections %d", errEntryInvalid, entry, len(sections))
	}

	return sections[0], sections[1], nil
}

// parseOptions parses querySection returning the baseQuery and query options.
func parseOptions(querySection string) (baseQuery string, options []svrquery.Option, err error) {
	options = make([]svrquery.Option, 0)
	protocolSections := strings.Split(querySection, ",")
	for i := 1; i < len(protocolSections); i++ {
		keyVal := strings.SplitN(protocolSections[i], "=", 2)
		if len(keyVal) != 2 {
			return "", nil, fmt.Errorf("key value pair invalid: %v", keyVal)

		}

		// Only support key at the moment.
		switch strings.ToLower(keyVal[0]) {
		case "key":
			options = append(options, svrquery.WithKey(keyVal[1]))
		}
	}
	return protocolSections[0], options, nil
}
