package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/multiplay/go-svrquery/lib/svrquery"
	"github.com/multiplay/go-svrquery/lib/svrsample"
	"github.com/multiplay/go-svrquery/lib/svrsample/common"
	"github.com/stretchr/testify/require"
)

var errOutput = errors.New("output failed")

type failingWriter struct {
	writes int
	failAt int
	short  bool
}

func (w *failingWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes == w.failAt {
		if w.short {
			return len(p) - 1, nil
		}
		return 0, errOutput
	}
	return len(p), nil
}

func TestWriter(t *testing.T) {
	for _, tc := range []struct {
		name  string
		items []BulkResponseItem
		json  string
	}{
		{"empty", nil, "[]\n"},
		{"one", []BulkResponseItem{{Address: "one"}}, "[{\"address\":\"one\"}]\n"},
		{"multiple", []BulkResponseItem{{Address: "one"}, {Error: "bad entry"}}, "[{\"address\":\"one\"},{\"error\":\"bad entry\"}]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			results := make(chan BulkResponseItem, len(tc.items))
			for _, item := range tc.items {
				results <- item
			}
			close(results)
			var output bytes.Buffer
			require.NoError(t, writer(&output, results))
			require.Equal(t, tc.json, output.String())
		})
	}

	for _, short := range []bool{false, true} {
		for _, failAt := range []int{1, 2, 3, 4, 5} {
			results := make(chan BulkResponseItem, 2)
			results <- BulkResponseItem{Address: "one"}
			results <- BulkResponseItem{Address: "two"}
			close(results)
			expected := errOutput
			if short {
				expected = io.ErrShortWrite
			}
			require.ErrorIs(t, writer(&failingWriter{failAt: failAt, short: short}, results), expected)
		}
	}
}

func TestWriterStreams(t *testing.T) {
	results := make(chan BulkResponseItem)
	r, w := io.Pipe()
	t.Cleanup(func() {
		require.NoError(t, r.Close())
		require.NoError(t, w.Close())
	})
	done := make(chan error, 1)
	go func() {
		done <- writer(w, results)
	}()
	decoded := make(chan error, 1)
	go func() {
		decoder := json.NewDecoder(r)
		if _, err := decoder.Token(); err != nil {
			decoded <- err
			return
		}
		var item BulkResponseItem
		if err := decoder.Decode(&item); err != nil {
			decoded <- err
			return
		}
		if item.Address != "streamed" {
			decoded <- fmt.Errorf("unexpected item: %+v", item)
			return
		}
		decoded <- nil
		_, err := io.Copy(io.Discard, r)
		decoded <- err
	}()
	results <- BulkResponseItem{Address: "streamed"}
	select {
	case err := <-decoded:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("first result was not written before the input closed")
	}
	close(results)
	require.NoError(t, <-done)
	require.NoError(t, w.Close())
	require.NoError(t, <-decoded)
}

func bulkFile(t *testing.T, contents string) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "bulk.txt")
	require.NoError(t, os.WriteFile(file, []byte(contents), 0600))
	return file
}

func TestQueryBulk(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		var output bytes.Buffer
		require.NoError(t, queryBulk(bulkFile(t, ""), &output))
		require.Equal(t, "[]\n", output.String())
	})
	t.Run("more than former limit", func(t *testing.T) {
		const count = 10001
		var output bytes.Buffer
		require.NoError(t, queryBulk(bulkFile(t, strings.Repeat("unsupported 127.0.0.1:1\n", count)), &output))
		var items []BulkResponseItem
		require.NoError(t, json.Unmarshal(output.Bytes(), &items))
		require.Len(t, items, count)
		for _, item := range items {
			require.Equal(t, "127.0.0.1:1", item.Address)
			require.Contains(t, item.Error, "unknown protocol")
		}
	})
	t.Run("best effort", func(t *testing.T) {
		var output bytes.Buffer
		require.NoError(t, queryBulk(bulkFile(t, "\nbad entry extra\nsqp,key 127.0.0.1:1\nsqp 127.0.0.1:invalid\n"), &output))
		var items []BulkResponseItem
		require.NoError(t, json.Unmarshal(output.Bytes(), &items))
		require.Len(t, items, 4)
		for _, item := range items {
			require.NotEmpty(t, item.Error)
			require.Nil(t, item.ServerInfo)
		}
	})
	t.Run("read error preserves partial results", func(t *testing.T) {
		var output bytes.Buffer
		err := queryBulk(bulkFile(t, "unsupported 127.0.0.1:1\n"+strings.Repeat("x", 65536)), &output)
		require.ErrorContains(t, err, "token too long")
		var items []BulkResponseItem
		require.NoError(t, json.Unmarshal(output.Bytes(), &items))
		require.Len(t, items, 1)
	})
	t.Run("producer and writer errors preserved", func(t *testing.T) {
		err := queryBulk(filepath.Join(t.TempDir(), "missing"), &failingWriter{failAt: 1})
		require.ErrorIs(t, err, os.ErrNotExist)
		require.ErrorIs(t, err, errOutput)
	})
	for _, failAt := range []int{1, 2, 3, 4} {
		t.Run(fmt.Sprintf("output failure %d", failAt), func(t *testing.T) {
			file := bulkFile(t, strings.Repeat("unsupported 127.0.0.1:1\n", 10001))
			done := make(chan error, 1)
			go func() { done <- queryBulk(file, &failingWriter{failAt: failAt}) }()
			select {
			case err := <-done:
				require.ErrorIs(t, err, errOutput)
			case <-time.After(5 * time.Second):
				t.Fatal("queryBulk deadlocked after output failure")
			}
		})
	}
}

func TestQueryBulkLocalSQP(t *testing.T) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	responder, err := svrsample.GetResponder("sqp", common.QueryState{
		CurrentPlayers: 3, MaxPlayers: 8, Map: "bulk-map",
	})
	require.NoError(t, err)
	stopped := make(chan error, 1)
	go func() {
		buf := make([]byte, 2048)
		for {
			n, peer, err := conn.ReadFromUDP(buf)
			if errors.Is(err, net.ErrClosed) {
				stopped <- nil
				return
			}
			if err == nil {
				var response []byte
				response, err = responder.Respond(peer.String(), buf[:n])
				if err == nil {
					_, err = conn.WriteToUDP(response, peer)
				}
			}
			if err != nil {
				stopped <- err
				return
			}
		}
	}()
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
		require.NoError(t, <-stopped)
	})
	const count = 3 * numWorkers
	line := "sqp " + conn.LocalAddr().String() + "\n"
	var output bytes.Buffer
	require.NoError(t, queryBulk(bulkFile(t, strings.Repeat(line, count)), &output))
	var items []BulkResponseItem
	require.NoError(t, json.Unmarshal(output.Bytes(), &items))
	require.Len(t, items, count)
	for _, item := range items {
		require.Empty(t, item.Error)
		require.Equal(t, conn.LocalAddr().String(), item.Address)
		require.Equal(t, &BulkResponseServerInfoItem{
			CurrentPlayers: 3, MaxPlayers: 8, Map: "bulk-map",
		}, item.ServerInfo)
	}
}

func TestProducerAndWorkerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	work := make(chan string)
	require.ErrorIs(t, producer(ctx, bulkFile(t, "sqp 127.0.0.1:1\n"), work), context.Canceled)
	_, ok := <-work
	require.False(t, ok)
	client, err := svrquery.NewBulkClient()
	require.NoError(t, err)
	worker(ctx, nil, nil, client)
}

func TestParseEntry(t *testing.T) {
	testCases := []struct {
		name       string
		input      string
		expQuery   string
		expAddress string
		expErr     error
	}{
		{
			name:       "ok",
			input:      "sqp 1.2.3.4:1234",
			expQuery:   "sqp",
			expAddress: "1.2.3.4:1234",
		},
		{
			name:   "empty line",
			input:  "",
			expErr: errNoItem,
		},
		{
			name:   "invalid entry",
			input:  "sqp 1.2.3.4:1234 extra",
			expErr: errEntryInvalid,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			query, addr, err := parseEntry(tc.input)
			if err != nil {
				require.ErrorIs(t, err, tc.expErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expQuery, query)
			require.Equal(t, tc.expAddress, addr)
		})
	}
}

func TestCreateClient(t *testing.T) {
	testCases := []struct {
		name     string
		query    string
		expQuery string
		expKey   string
		expErr   error
	}{
		{
			name:     "ok",
			query:    "tf2e",
			expQuery: "tf2e",
		},
		{
			name:     "with_key",
			query:    "tf2e,key=val",
			expKey:   "val",
			expQuery: "tf2e",
		},
		{
			name:     "with_unsupported_other",
			query:    "tf2e,other=val",
			expQuery: "tf2e",
		},
		{
			name:   "invalid entry",
			query:  "tf2e,key",
			expErr: errEntryInvalid,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			baseQuery, options, err := parseOptions(tc.query)
			if tc.expErr != nil {
				require.ErrorIs(t, err, tc.expErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expQuery, baseQuery)

			// Validate key setting
			if tc.expKey != "" {
				require.Len(t, options, 1)
				c := svrquery.Client{}
				require.NoError(t, options[0](&c))
				require.Equal(t, tc.expKey, c.Key())
			}
			require.NotNil(t, options)
		})
	}
}
