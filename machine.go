// Copyright 2021 The reconnx Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package reconnx

import (
	"math"
	"sync"
)

const (
	// DefaultHistoricalSamples is the default value used for number of
	// historical samples if the HistoricalSamples field of a
	// MachineConfig is zero.
	DefaultHistoricalSamples = 10

	// DefaultRecentSamples is the default value used for number of
	// recent samples if the RecentSamples field of a MachineConfig is
	// zero.
	DefaultRecentSamples = 3

	badWindowSizeMsg = "reconnx: window size must be positive"
)

// A State represents the current state of a Machine.
type State int

const (
	// Watching indicates that a Machine is actively watching new data,
	// evaluating whether it should transition to the Closing state.
	Watching State = iota

	// Closing indicates that a Machine is actively closing connections,
	// and may transition to the Resting or Watching states.
	Closing

	// Resting indicates that a Machine is in a resting period where it
	// passively accumulates new data but does not change states. Once
	// the resting period is over, the machine will transition either to
	// the Watching state, if the data are good, or the Closing state,
	// if the data indicate connections should be closed.
	Resting
)

func (s State) String() string {
	switch s {
	case Watching:
		return "Watching"
	case Closing:
		return "Closing"
	case Resting:
		return "Resting"
	default:
		return ""
	}
}

// A Machine is a simple generic state machine for deciding whether to
// close connections.
//
// The state machine can be used independently of the plugin itself and
// thus both the Machine interface and the NewMachine function are
// exported.
type Machine interface {
	// State reports the current state of the state machine.
	State() State

	// Next receives a new data point into the state machine and returns
	// the next and previous states.
	//
	// The new data point consists of a value and a closed flag. The
	// value is averaged with other recent values and checked against
	// the MachineConfig's absolute and percent thresholds. The closed
	// flag indicates whether the HTTP connection that generated the
	// value was closed.
	//
	// If the new data point did cause not a state transition, the next
	// state is equal to the previous state.
	Next(value float64, closed bool) (next State, prev State)
}

type avgWindow struct {
	values []float64
	sum    float64
	index  int
}

func newAvgWindow(n uint, def float64) avgWindow {
	if n < 1 {
		panic("reconnx: window size must be positive")
	}
	values := make([]float64, n)
	sum := 0.0
	for i := uint(0); i < n; i++ {
		values[i] = def
		sum += def
	}
	return avgWindow{values: values, sum: sum}
}

func (aw *avgWindow) Avg() float64 {
	return aw.sum / float64(len(aw.values))
}

func (aw *avgWindow) Push(value float64) (dropped float64) {
	dropped = aw.values[aw.index]
	aw.values[aw.index] = value
	aw.index++
	aw.sum -= dropped
	aw.sum += value
	if aw.index >= len(aw.values) {
		aw.index = 0
	}
	return
}

type machine struct {
	state        State
	historical   avgWindow
	recent       avgWindow
	closedStreak uint
	closedCount  uint
	restCount    uint
	config       MachineConfig
	lock         sync.RWMutex
}

// A MachineConfig specifies how to configure a new Machine.
type MachineConfig struct {
	// HistoricalSamples is the number of samples to include in the
	// historical average. If zero, DefaultHistoricalSamples is used.
	HistoricalSamples uint

	// RecentSamples is the number of samples to include in the recent
	// average. If zero, DefaultRecentSamples is used.
	RecentSamples uint

	// AbsThreshold specifies the absolute value threshold for closing
	// connections. If the recent average is greater than AbsThreshold,
	// the Machine will transition to the Closing state.
	//
	// If AbsThreshold is zero or negative, there is no absolute
	// threshold and the absolute value of the recent average will not
	// cause the Machine to transition to the Closing state.
	//
	// Note that if AbsThreshold is positive, the Machine's Next method
	// clamps to AbsThreshold (AbsThreshold is the maximum possible
	// value) before including them in the recent or historical average.
	// This prevents a single outlier value from skewing the recent or
	// historical averages. This clamping behavior also affects the
	// percentage calculation if both AbsThreshold and PctThreshold are
	// positive, but does not affect it if AbsThreshold is zero or
	// negative.
	AbsThreshold float64

	// PctThreshold specifies the percentage threshold for closing
	// connections. If the recent average is at least PctThreshold %
	// greater than the historical average, the Machine will transition
	// to the Closing state.
	//
	// If PctThreshold is zero or negative, there is no percentage
	// threshold and a percentage increase of the recent average over
	// the historical average will not cause the Machine to transition
	// to the Closing state.
	//
	// The Units for PctThreshold are "percentage points". Therefore the
	// value 1.0 represents a 1% increase, 20.0 represents a 20%
	// increase, and 120.0 represents a 120% increase.
	//
	// The percentage calculation can be affected by the AbsThreshold
	// value in the following way. If AbsThreshold is positive, then
	// all new values are clamped to AbsThreshold before being included
	// in the average. This prevents outliers from skewing the averages,
	// as this can cause legitimate anomalies to be missed.
	PctThreshold float64

	// ClosingStreak specifies the number of consecutive HTTP connections
	// that must be closed in order to transition out of the Closing
	// state.
	//
	// If this value is zero, the Machine will never enter the Closing
	// state.
	//
	// Even if the closing streak is not reached, the Machine will
	// transition out of the Closing state if the total closed
	// connection count (ClosingCount) is reached first.
	ClosingStreak uint

	// ClosingCount specifies the number of total (not necessarily
	// consecutive) HTTP connections that must be closed in order to
	// transition out of the Closing state.
	//
	// If this value is zero, the Machine will never enter the Closing
	// state.
	//
	// The Machine may transition out of the Closing state before the
	// closed connection count reaches ClosingCount if the closed
	// connection streak (ClosingStreak) is reached first.
	ClosingCount uint

	// RestingCount specifies the number of data points for which the
	// machine should "rest" in the Resting state after transitioning
	// out of the Closing state, and before transitioning back to either
	// Watching or Closing.
	//
	// If this value is zero, the Machine will never enter the Resting
	// state.
	//
	// The purpose of this value is to help prevent "infinite loop"
	// cases where persistent bad performance results in every
	// connection being closed all the time. Since HTTP connection setup
	// isn't free, and the TLS handshake required for HTTPS connections
	// is CPU-intensive both for the client and the server, the Resting
	// state exists as a circuit breaker to help prevent brownout
	// scenarios when a remote host is having a bad day.
	RestingCount uint
}

// NewMachine constructs a new Machine with the given configuration.
func NewMachine(config MachineConfig) Machine {
	historicalLen := valOrDef(config.HistoricalSamples, DefaultHistoricalSamples)
	recentLen := valOrDef(config.RecentSamples, DefaultRecentSamples)
	return &machine{
		historical: newAvgWindow(historicalLen, config.AbsThreshold),
		recent:     newAvgWindow(recentLen, config.AbsThreshold),
		config:     config,
	}
}

func (m *machine) State() State {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.state
}

func (m *machine) Next(value float64, closed bool) (next State, prev State) {
	m.lock.Lock()
	defer m.lock.Unlock()

	prev = m.state
	m.shift(value)
	switch prev {
	case Watching:
		m.watching()
	case Closing:
		m.closing(closed)
	case Resting:
		m.resting()
	default:
		panic("reconnx: unknown state")
	}

	next = m.state
	return
}

func (m *machine) shift(value float64) {
	if m.config.AbsThreshold > 0.0 {
		value = math.Min(value, m.config.AbsThreshold)
	}
	dropped := m.recent.Push(value)
	m.historical.Push(dropped)
}

func (m *machine) watching() {
	recentAvg := m.recent.Avg()
	if m.config.AbsThreshold > 0.0 && recentAvg >= m.config.AbsThreshold {
		m.state = Closing
	} else if m.config.PctThreshold > 0.0 && recentAvg >= m.historical.Avg()*((100.0+m.config.PctThreshold)/100.0) {
		m.state = Closing
	}

	if m.state == Closing {
		m.closing(false)
	}
}

func (m *machine) closing(closed bool) {
	if closed {
		m.closedCount++
		m.closedStreak++
	} else {
		m.closedStreak = 0
	}
	if m.closedStreak >= m.config.ClosingStreak || m.closedCount >= m.config.ClosingCount {
		m.closedCount = 0
		m.closedStreak = 0
		if m.config.RestingCount > 0 {
			m.state = Resting
		} else {
			m.state = Watching
		}
	}
}

func (m *machine) resting() {
	m.restCount++
	if m.restCount >= m.config.RestingCount {
		m.restCount = 0
		m.state = Watching
		m.watching()
	}
}

func valOrDef(val, def uint) uint {
	if val > 0 {
		return val
	}

	return def
}
