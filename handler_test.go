// Copyright 2021 The reconnx Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package reconnx

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/stretchr/testify/require"

	"github.com/gogama/httpx/request"
	"github.com/stretchr/testify/assert"

	"github.com/gogama/httpx"
)

func TestHandler_Handler(t *testing.T) {
	t.Run("UnsupportedEvent", func(t *testing.T) {
		unsupported := []httpx.Event{
			httpx.BeforeReadBody,
			httpx.AfterAttemptTimeout,
			httpx.AfterPlanTimeout,
			httpx.AfterExecutionEnd,
		}

		for _, evt := range unsupported {
			t.Run(evt.String(), func(t *testing.T) {
				h := &handler{}
				assert.PanicsWithValue(t, unsupportedEventMsg, func() {
					h.Handle(evt, &request.Execution{})
				})
			})
		}
	})
	t.Run("SupportedEvent", func(t *testing.T) {
		t.Run("BeforeExecutionStart", testBeforeExecutionStart)
		t.Run("BeforeAttempt", testBeforeAttempt)
		t.Run("AfterAttempt", testAfterAttempt)
	})
}

func testBeforeExecutionStart(t *testing.T) {
	h, l := newHandlerWithLogger(t)
	e := request.Execution{}

	h.Handle(httpx.BeforeExecutionStart, &e)

	l.AssertExpectations(t)
	v := e.Value(executionStateKey)
	require.IsType(t, &executionState{}, v)
	es := v.(*executionState)
	assert.Equal(t, &executionState{}, es)
}

func testBeforeAttempt(t *testing.T) {
	t.Run("MissingExecutionPlan", func(t *testing.T) {
		h, l := newHandlerWithLogger(t)
		l.On("Printf", missingExecutionPlanMsg).Once()

		h.Handle(httpx.BeforeAttempt, &request.Execution{})

		l.AssertExpectations(t)
	})
	t.Run("MissingExecutionRequest", func(t *testing.T) {
		h, l := newHandlerWithLogger(t)
		l.On("Printf", missingExecutionRequestMsg).Once()

		h.Handle(httpx.BeforeAttempt, &request.Execution{Plan: &request.Plan{}})

		l.AssertExpectations(t)
	})
	t.Run("MissingExecutionState", func(t *testing.T) {
		h, l := newHandlerWithLogger(t)
		l.On("Printf", missingExecutionStateMsg)

		h.Handle(httpx.BeforeAttempt, &request.Execution{
			Plan:    &request.Plan{},
			Request: &http.Request{},
		})

		l.AssertExpectations(t)
	})
	t.Run("AttemptMismatch", func(t *testing.T) {
		h, l := newHandlerWithLogger(t)
		var errMsg string
		l.On("Printf", mock.AnythingOfType("string"), mock.AnythingOfType("[]interface {}")).
			Run(renderPrintf(&errMsg)).
			Once()
		e := &request.Execution{
			Plan:    &request.Plan{},
			Request: &http.Request{},
			Attempt: 5,
		}
		e.SetValue(executionStateKey, &executionState{})

		h.Handle(httpx.BeforeAttempt, e)

		l.AssertExpectations(t)
		assert.Equal(t, "reconnx: ERROR: unexpected attempt start (5)", errMsg)
	})
	t.Run("NoLatencyStateMachineForHost", func(t *testing.T) {
		h, l := newHandlerWithLogger(t)
		p, err := request.NewPlan("", "http://foo.com", nil)
		require.NoError(t, err)
		e := &request.Execution{
			Plan:    p,
			Request: &http.Request{},
		}
		e.SetValue(executionStateKey, &executionState{})

		h.Handle(httpx.BeforeAttempt, e)

		l.AssertExpectations(t)
		assert.Contains(t, h.hostLatency, "foo.com")
		m := h.hostLatency["foo.com"]
		require.IsType(t, &machine{}, m)
		assert.Equal(t, h.Config.Latency, m.(*machine).config)
	})
	t.Run("ExistingLatencyStateMachineForHost", func(t *testing.T) {
		t.Run("NotClosingState", func(t *testing.T) {
			h, l := newHandlerWithLogger(t)
			p, err := request.NewPlan("", "https://bar.org", nil)
			require.NoError(t, err)
			e := &request.Execution{
				Plan:    p,
				Request: &http.Request{},
			}
			e.SetValue(executionStateKey, &executionState{})
			m1 := &machine{}
			h.hostLatency["bar.org"] = m1

			l.AssertExpectations(t)

			h.Handle(httpx.BeforeAttempt, e)
			m2 := h.hostLatency["bar.org"]
			require.IsType(t, &machine{}, m2)
			assert.Same(t, m1, m2.(*machine))
			assert.False(t, e.Request.Close)
		})
		t.Run("ClosingState", func(t *testing.T) {
			h, l := newHandlerWithLogger(t)
			l.On("Printf", "reconnx: a connection to %s will be closed after attempt %d ends", []interface{}{"baz.edu", 1}).Once()
			p, err := request.NewPlan("", "https://baz.edu", nil)
			require.NoError(t, err)
			e := &request.Execution{
				Plan:    p,
				Request: &http.Request{},
				Attempt: 1,
			}
			e.SetValue(executionStateKey, &executionState{
				attemptStart: make([]time.Time, 1),
			})
			m1 := &machine{
				state: Closing,
			}
			h.hostLatency["baz.edu"] = m1

			h.Handle(httpx.BeforeAttempt, e)

			l.AssertExpectations(t)
			m2 := h.hostLatency["baz.edu"]
			require.IsType(t, &machine{}, m2)
			assert.Same(t, m1, m2.(*machine))
			assert.True(t, e.Request.Close)
		})
	})
}

func testAfterAttempt(t *testing.T) {
	t.Run("MissingExecutionPlan", func(t *testing.T) {
		h, l := newHandlerWithLogger(t)
		l.On("Printf", missingExecutionPlanMsg).Once()

		h.Handle(httpx.AfterAttempt, &request.Execution{})

		l.AssertExpectations(t)
	})
	t.Run("MissingExecutionState", func(t *testing.T) {
		h, l := newHandlerWithLogger(t)
		l.On("Printf", missingExecutionStateMsg)

		h.Handle(httpx.AfterAttempt, &request.Execution{
			Plan:    &request.Plan{},
			Request: &http.Request{},
		})

		l.AssertExpectations(t)
	})
	t.Run("UnexpectedAttemptEnd", func(t *testing.T) {
		h, l := newHandlerWithLogger(t)
		var errMsg string
		l.On("Printf", mock.AnythingOfType("string"), mock.AnythingOfType("[]interface {}")).
			Run(renderPrintf(&errMsg)).
			Once()
		e := &request.Execution{
			Plan:    &request.Plan{},
			Request: &http.Request{},
		}
		e.SetValue(executionStateKey, &executionState{})

		h.Handle(httpx.AfterAttempt, e)

		l.AssertExpectations(t)
		assert.Equal(t, "reconnx: ERROR: unexpected attempt end (0)", errMsg)
	})
	t.Run("MissingLatencyStateMachineForHost", func(t *testing.T) {
		h, l := newHandlerWithLogger(t)
		var errMsg string
		l.On("Printf", mock.AnythingOfType("string"), mock.AnythingOfType("[]interface {}")).
			Run(renderPrintf(&errMsg)).
			Once()
		e := &request.Execution{
			Plan:    &request.Plan{Host: "foo.bar"},
			Request: &http.Request{},
		}
		e.SetValue(executionStateKey, &executionState{
			attemptStart: []time.Time{time.Now()},
		})

		h.Handle(httpx.AfterAttempt, e)

		l.AssertExpectations(t)
		assert.Equal(t, "reconnx: ERROR: missing latency state machine for host (foo.bar)", errMsg)
	})
	t.Run("ExistingLatencyStateMachineForHost", func(t *testing.T) {
		t.Run("NoStateChange", func(t *testing.T) {
			for _, closed := range []bool{false, true} {
				t.Run(fmt.Sprintf("closed:%t", closed), func(t *testing.T) {
					h, l := newHandlerWithLogger(t)
					m := newMockMachine(t)
					m.
						On("Next", mock.AnythingOfType("float64"), closed).
						Return(Watching, Watching).
						Once()
					e := &request.Execution{
						Plan: &request.Plan{Host: "spam"},
						Request: &http.Request{
							Close: closed,
						},
						Attempt: 1,
					}
					e.SetValue(executionStateKey, &executionState{
						attemptStart: []time.Time{time.Now(), time.Now()},
					})
					h.hostLatency["spam"] = m

					h.Handle(httpx.AfterAttempt, e)

					l.AssertExpectations(t)
					m.AssertExpectations(t)
				})
			}
		})
		t.Run("StateChange", func(t *testing.T) {
			for _, closed := range []bool{false, true} {
				t.Run(fmt.Sprintf("closed:%t", closed), func(t *testing.T) {
					h, l := newHandlerWithLogger(t)
					m := newMockMachine(t)
					var infoMsg string
					l.On("Printf", mock.AnythingOfType("string"), mock.AnythingOfType("[]interface {}")).
						Run(renderPrintf(&infoMsg)).
						Once()
					m.
						On("Next", mock.AnythingOfType("float64"), closed).
						Return(Closing, Resting).
						Once()
					e := &request.Execution{
						Plan: &request.Plan{Host: "wham!"},
						Request: &http.Request{
							Close: closed,
						},
					}
					e.SetValue(executionStateKey, &executionState{
						attemptStart: []time.Time{time.Now()},
					})
					h.hostLatency["wham!"] = m

					h.Handle(httpx.AfterAttempt, e)

					l.AssertExpectations(t)
					m.AssertExpectations(t)
					assert.Equal(t, "reconnx: after attempt 0, host wham! state changed from Closing to Resting", infoMsg)
				})
			}
		})
	})
}

func renderPrintf(target *string) func(args mock.Arguments) {
	return func(args mock.Arguments) {
		format := args.Get(0).(string)
		a := args.Get(1).([]interface{})
		*target = fmt.Sprintf(format, a...)
	}
}

func newHandlerWithLogger(t *testing.T) (*handler, *mockLogger) {
	l := newMockLogger(t)
	return &handler{
		Config: Config{
			Logger: l,
		},
		hostLatency: map[string]Machine{},
	}, l
}
