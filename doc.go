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
			AbsThreshold:   1500.0, // Close conns to host if recent latency > 1500ms
			PctThreshold:   200.0,  // Close conns to host if recent latency > 200% of historical
			ClosingStreak:  5,      // Stop closing conns to host once 5 in a row were closed...
			ClosingCount:   20,     // ...OR... Stop closing conns to host once 20 in total were closed
		},
	}
	reconnx.OnClient(cl, cfg)       // Install reconnx plugin

If you need to install reconnx directly onto an httpx.HandlerGroup, use
the OnHandlers function.
*/
package reconnx
