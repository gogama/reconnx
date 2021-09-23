// Copyright 2021 The reconnx Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package reconnx

import (
	"sync"
	"time"

	"github.com/gogama/httpx"
	"github.com/gogama/httpx/request"
)

type handler struct {
	Config
	hostLatency     map[string]Machine
	hostLatencyLock sync.RWMutex
}

func (h *handler) Handle(evt httpx.Event, e *request.Execution) {
	switch evt {
	case httpx.BeforeExecutionStart:
		beforeExecutionStart(e)
	case httpx.BeforeAttempt:
		beforeAttempt(h, e)
	case httpx.AfterAttempt:
		afterAttempt(h, e)
	default:
		panic(unsupportedEventMsg)
	}
}

type executionStateKeyType int

var executionStateKey = new(executionStateKeyType)

type executionState struct {
	attemptStart []time.Time
}

func beforeExecutionStart(e *request.Execution) {
	e.SetValue(executionStateKey, &executionState{})
}

func beforeAttempt(h *handler, e *request.Execution) {
	// Synchronization is not required because httpx guarantees that all
	// event handlers are called on the main goroutine.

	// Get target the host name.
	host, ok := getExecutionHost(h, e)
	if !ok {
		return
	}

	// Get the request.
	r := e.Request
	if r == nil {
		h.Logger.Printf(missingExecutionRequestMsg)
		return
	}

	// Record the attempt start time.
	es := getExecutionState(h, e)
	if es == nil {
		return
	}
	if len(es.attemptStart) != e.Attempt {
		h.Logger.Printf("reconnx: ERROR: unexpected attempt start (%d)", e.Attempt)
		return
	}
	es.attemptStart = append(es.attemptStart, time.Now())

	// Check the state machine for this host to see if it the connection
	// should be closed when the attempt finishes.
	sm := getOrCreateHostLatencyStateMachine(h, host)
	if sm.State() == Closing {
		h.Logger.Printf("reconnx: a connection to %s will be closed after attempt %d ends", host, e.Attempt)
		r.Close = true
	}
}

func afterAttempt(h *handler, e *request.Execution) {
	// Synchronization is not required because httpx guarantees that all
	// event handlers are called on the main goroutine.

	// Get target the host name.
	host, ok := getExecutionHost(h, e)
	if !ok {
		return
	}

	// Determine attempt end time.
	es := getExecutionState(h, e)
	if es == nil {
		return
	}
	if e.Attempt >= len(es.attemptStart) {
		h.Logger.Printf("reconnx: ERROR: unexpected attempt end (%d)", e.Attempt)
		return
	}
	d := time.Now().Sub(es.attemptStart[e.Attempt])

	// Push the attempt time into the host latency state machine.
	sm := getHostLatencyStateMachine(h, host)
	if sm == nil {
		h.Logger.Printf("reconnx: ERROR: missing latency state machine for host (%s)", host)
		return
	}
	next, prev := sm.Next(float64(d.Milliseconds()), e.Request.Close)
	if prev != next {
		h.Logger.Printf("reconnx: after attempt %d, host %s state changed from %s to %s", e.Attempt, host, prev, next)
	}
}

func getExecutionHost(h *handler, e *request.Execution) (string, bool) {
	p := e.Plan
	if p == nil {
		h.Logger.Printf(missingExecutionPlanMsg)
		return "", false
	}

	return p.Host, true
}

func getExecutionState(h *handler, e *request.Execution) *executionState {
	es, _ := e.Value(executionStateKey).(*executionState)
	if es == nil {
		h.Logger.Printf(missingExecutionStateMsg)
		return nil
	}

	return es
}

func getHostLatencyStateMachine(h *handler, host string) Machine {
	h.hostLatencyLock.RLock()
	defer h.hostLatencyLock.RUnlock()
	return h.hostLatency[host]
}

func getOrCreateHostLatencyStateMachine(h *handler, host string) Machine {
	h.hostLatencyLock.Lock()
	defer h.hostLatencyLock.Unlock()
	if sm, ok := h.hostLatency[host]; ok {
		return sm
	}
	sm := NewMachine(h.Latency)
	h.hostLatency[host] = sm
	return sm
}

const (
	unsupportedEventMsg        = "reconnx: unsupported event"
	missingExecutionPlanMsg    = "reconnx: ERROR: missing execution plan"
	missingExecutionRequestMsg = "reconnx: ERROR: missing execution request"
	missingExecutionStateMsg   = "reconnx: ERROR: missing execution state"
)
