// Copyright 2021 The reconnx Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

/*
Package reconnx dynamically closes connections to slow hosts, improving
HTTP performance for the httpx library's robust HTTP client. See
https://github.com/gogama/httpx.

Use the OnClient function to install reconnx in any httpx.Client:

	cl := &httpx.Client{}           // Create robust HTTP client
	cfg := reconnx.Config{
		Latency: reconnx.MachineConfig{
			AbsoluteMax: 1500.0,    // Close conns to host if recent latency > 1500ms
			PercentMax:  200.0,     // Close conns to host if recent latency > 200% of historical
		},
	}
	reconnx.OnClient(cl, cfg)       // Install reconnx plugin

Use the OnHandlers function to install reconnx directly onto an
httpx.HandlerGroup.
*/
package reconnx
