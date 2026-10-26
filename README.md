# go-svrquery [![Go Report Card](https://goreportcard.com/badge/github.com/multiplay/go-svrquery)](https://goreportcard.com/report/github.com/multiplay/go-svrquery) [![License](https://img.shields.io/badge/license-BSD-blue.svg)](https://github.com/rocketsciencegg/go-svrquery/blob/main/LICENSE) [![GoDoc](https://godoc.org/github.com/multiplay/go-svrquery?status.svg)](https://godoc.org/github.com/multiplay/go-svrquery) [![Build Status](https://github.com/rocketsciencegg/go-svrquery/actions/workflows/build-test.yml/badge.svg?branch=main)](https://github.com/rocketsciencegg/go-svrquery/actions/workflows/build-test.yml)

go-svrquery is a [Go](http://golang.org/) client for talking to game servers using various query protocols.

Features
--------
* Support for various game server query protocol's including:
** SQP, TF2E

Installation
------------
```sh
go get -u github.com/multiplay/go-svrquery
```

Examples
--------

Using go-svrquery is simple just create a client and then send commands e.g.
```go
package main

import (
	"log"
	"time"

	"github.com/multiplay/go-svrquery/lib/svrquery"
)

func main() {
	c, err := svrquery.NewClient("tf2e", "192.168.1.102:10011")
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	r, err := c.Query()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%#v\n", r)
}
```

CLI
-------------
A cli is available in [GitHub releases](https://github.com/rocketsciencegg/go-svrquery/releases) and also at https://github.com/rocketsciencegg/go-svrquery/tree/main/cmd/cli

This enables you make queries to servers using the specified protocol, and returns the response in pretty json.

### Client

```
./go-svrquery -addr localhost:12121 -proto sqp
{
        "version": 1,
        "address": "localhost:12121",
        "server_info": {
                "current_players": 1,
                "max_players": 2,
                "server_name": "Name",
                "game_type": "Game Type",
                "build_id": "",
                "map": "Map",
                "port": 1000
        }
}
```

### SQP performance reporting

Use `-proto sqp-v2` to request SQP version 2 server information and performance
data. The `performance_info` JSON object contains zero-based `flag_N` booleans
and `gauge_N` numbers, for example:

```json
{"flag_0": true, "flag_1": false, "gauge_0": 60}
```

SQP v2 reuses the v1 metrics request bit for its flags-and-gauges chunk.
The default `sqp` protocol remains version 1 and its `metrics` format is unchanged.
Responses must use the requested version (1 or 2); unsupported or mismatched
versions are rejected before decoding the response body.
The CLI sample server supports both versions and provides example performance
flags and gauges for v2 queries. The default `sqp` client requests server
information only, preserving legacy behavior. Embedded responders can set
`PerformanceFlags` and `PerformanceGauges` in `common.QueryState`; these are empty
unless supplied.

### Example Server

This tool also provides the ability to start a very basic sample server using a given protocol.

Supported sample-server protocols are `sqp`, `sqp-v2`, `tf2`, `tf2e`, `tf2e-v7`, and
`tf2e-v8`. The TF2E sample responders use unkeyed queries; authentication and
encryption are not implemented. The `tf2` responder emits the legacy version 1
format, which the query client does not currently support.

```
./go-svrquery -server :12121 -proto sqp
Starting sample server using protocol sqp on :12121
```

### Bulk queries

Use `./go-svrquery -file servers.txt` to query a file containing one
`protocol address` entry per line. For example:

```text
sqp 127.0.0.1:12121
tf2e-v8,key=BASE64_KEY 127.0.0.1:37015
```

Results are streamed to standard output as a single JSON array in completion
order, not input order. The worker pool and queues are bounded; the number of
entries is not capped. Each input line produces a result, including blank or
malformed lines, which produce an `error` and may have no `address`. Query and
entry errors do not stop other entries. Input read errors return a nonzero exit
status after already queued entries have been processed; output errors stop
further processing and return a nonzero exit status. Input lines must fit within
Go's default scanner limit (approximately 64 KiB).

Library callers can reuse `svrquery.NewBulkClient` for sequential queries.
Constructor options are defaults; options passed to `Query` apply only to that
request. Each query releases its UDP socket before returning. A bulk client must
not be shared by concurrent goroutines.

Documentation
-------------
- [GoDoc API Reference](http://godoc.org/github.com/multiplay/go-svrquery).

License
-------
go-svrquery is available under the [BSD 2-Clause License](https://opensource.org/licenses/BSD-2-Clause).
```
