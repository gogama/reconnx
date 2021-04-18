reconnx - Reconnect pooled HTTP connections when they get slow (httpx plugin)
=============================================================================

[![Build Status](https://travis-ci.com/gogama/reconnx.svg)](https://travis-ci.com/gogama/reconnx) [![Go Report Card](https://goreportcard.com/badge/github.com/gogama/reconnx)](https://goreportcard.com/report/github.com/gogama/reconnx) [![PkgGoDev](https://pkg.go.dev/badge/github.com/gogama/reconnx)](https://pkg.go.dev/github.com/gogama/reconnx)

Package reconnx is a Go-language plugin for the
[httpx](https://github.com/gogama/httpx) robust HTTP framework. The reconnx
plugin closes pooled HTTP connections when they experience degraded performance,
resulting in fresh replacement connections which may have substantially better
performance.

**Use Case**: Web services often use a DNS load balancing system, such as
Amazon Route53 or Azure Traffic Manager, to keep their DNS pointing at healthy
hosts. But the default GoLang HTTP client pools HTTP connections, so old
connections to bad hosts stay alive. The reconnx plugin closes these old
connections when they start to perform badly, allowing your service to re-query
DNS and shift over to healthy hosts.

---

Getting Started
===============

Install reconnx:

```sh
$ go get github.com/gogama/reconnx
```

Configure the reconnx plugin on an `httpx.Client` to begin reconnecting bad
connections:

```go
package main

import (
	"github.com/gogama/httpx"
	"github.com/gogama/reconnx"
)

func main() {
	client := &httpx.Client{}
	reconnx.OnClient(client, reconnx.Config{
		Latency: reconnx.MachineConfig{
			// If the average time taken by "recent" requests to a particular
			// hostname is more than 1000.0 milliseconds, start closing
			// connections to that hostname.
			AbsThreshold: 1000.0,
			// If the average time taken by "recent" requests to a particular
			// hostname is more than 20.0% slower than the longer-term average
			// for that hostname, start closing connections to that hostname.
			PctThreshold: 20.0,
			// Stop closing connections for a specific hostname once 3 in a row
			// have been closed, or once 5 have been closed in total, whichever
			// happens first.
			ClosingStreak: 3,
			ClosingCount:  5,
		},
	})

	// Use the httpx HTTP client.
	// ...
	// ...
}
```

---

License
=======

This project is licensed under the terms of the MIT License.

Acknowledgements
================

Developer happiness on this project was boosted by JetBrains' generous donation
of an [open source license](https://www.jetbrains.com/opensource/) for their
lovely GoLand IDE. ❤
