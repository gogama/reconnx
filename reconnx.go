// Copyright 2021 The reconnx Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package reconnx

import "github.com/gogama/httpx"

const (
	nilClientMsg       = "reconnx: nil client"
	nilHandlerGroupMsg = "reconnx: nil handler group"
)

type Config struct {
	Logger  Logger
	Latency MachineConfig
}

// OnClient installs the reconnx plugin onto an httpx.Client.
//
// If client's current handler group is nil, OnClient creates a new
// handler group, sets it as client's current handler group, and
// proceeds to install X-Ray support into the handler group. If the
// handler group is not nil, OnClient adds the reconnx plugin into the
// existing handler group. (Be aware of this behavior if you are sharing
// a handler group among multiple clients.)
func OnClient(client *httpx.Client, config Config) *httpx.Client {
	if client == nil {
		panic(nilClientMsg)
	}

	handlers := client.Handlers
	if handlers == nil {
		handlers = &httpx.HandlerGroup{}
		client.Handlers = handlers
	}

	OnHandlers(handlers, config)

	return client
}

// OnHandlers installs the reconnx plugin onto an httpx.HandlerGroup.
//
// The handler group may not be nil - if it is, a panic will ensue.
func OnHandlers(handlers *httpx.HandlerGroup, config Config) *httpx.HandlerGroup {
	if handlers == nil {
		panic(nilHandlerGroupMsg)
	}

	if config.Logger == nil {
		config.Logger = NopLogger{}
	}

	handler := &handler{
		Config:      config,
		hostLatency: map[string]Machine{},
	}
	handlers.PushBack(httpx.BeforeExecutionStart, handler)
	handlers.PushBack(httpx.BeforeAttempt, handler)
	handlers.PushBack(httpx.AfterAttempt, handler)

	return handlers
}
