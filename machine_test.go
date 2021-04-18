// Copyright 2021 The reconnx Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package reconnx

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
)

func TestState_String(t *testing.T) {
	t.Run("Simple", func(t *testing.T) {
		assert.Equal(t, "Watching", Watching.String())
		assert.Equal(t, "Closing", Closing.String())
		assert.Equal(t, "Resting", Resting.String())
		assert.Equal(t, "", State(-1).String())
	})
	t.Run("Fmt", func(t *testing.T) {
		assert.Equal(t, "Watching", fmt.Sprintf("%s", Watching))
	})
}

func TestAvgWindow(t *testing.T) {
	t.Run("New", func(t *testing.T) {
		t.Run("Zero.Len", func(t *testing.T) {
			assert.PanicsWithValue(t, badWindowSizeMsg, func() {
				newAvgWindow(0, 1.0)
			})
		})
		t.Run("Positive.Len", func(t *testing.T) {
			aw := newAvgWindow(1, 2.0)
			assert.Equal(t, []float64{2.0}, aw.values)
			assert.Equal(t, 2.0, aw.sum)
			assert.Equal(t, 0, aw.index)
		})
	})
	t.Run("PushAndAvg", func(t *testing.T) {
		t.Run("Size.One", func(t *testing.T) {
			aw := newAvgWindow(1, 1.0)
			assert.Equal(t, 1.0, aw.Avg())

			var dropped float64

			dropped = aw.Push(1.0)
			assert.Equal(t, 1.0, dropped)
			assert.Equal(t, 1.0, aw.Avg())

			dropped = aw.Push(-2.0)
			assert.Equal(t, 1.0, dropped)
			assert.Equal(t, -2.0, aw.Avg())

			dropped = aw.Push(0.0)
			assert.Equal(t, -2.0, dropped)
			assert.Equal(t, 0.0, aw.Avg())
		})
		t.Run("Size.Two", func(t *testing.T) {
			aw := newAvgWindow(2, 0.0)
			assert.Equal(t, 0.0, aw.Avg())

			var dropped float64

			dropped = aw.Push(1.0)
			assert.Equal(t, 0.0, dropped)
			assert.Equal(t, 0.5, aw.Avg())

			dropped = aw.Push(-2.0)
			assert.Equal(t, 0.0, dropped)
			assert.Equal(t, -0.5, aw.Avg())

			dropped = aw.Push(5.0)
			assert.Equal(t, 1.0, dropped)
			assert.Equal(t, 1.5, aw.Avg())

			dropped = aw.Push(5.0)
			assert.Equal(t, -2.0, dropped)
			assert.Equal(t, 5.0, aw.Avg())
		})
	})
}

func TestMachine(t *testing.T) {
	t.Run("New", func(t *testing.T) {
		t.Run("DefaultHistoryLen", func(t *testing.T) {
			m := NewMachine(MachineConfig{
				RecentLen: 123,
			})

			require.NotNil(t, m)
			require.IsType(t, &machine{}, m)
			m2 := m.(*machine)
			assert.Len(t, m2.history.values, DefaultHistoryLen)
			assert.Len(t, m2.recent.values, 123)
		})
		t.Run("DefaultRecentLen", func(t *testing.T) {
			m := NewMachine(MachineConfig{
				HistoryLen: 456,
			})

			require.NotNil(t, m)
			require.IsType(t, &machine{}, m)
			m2 := m.(*machine)
			assert.Len(t, m2.history.values, 456)
			assert.Len(t, m2.recent.values, DefaultRecentLen)
		})
		t.Run("ExplicitLens", func(t *testing.T) {
			m := NewMachine(MachineConfig{
				HistoryLen: 11,
				RecentLen:  22,
			})

			require.NotNil(t, m)
			require.IsType(t, &machine{}, m)
			m2 := m.(*machine)
			assert.Len(t, m2.history.values, 11)
			assert.Len(t, m2.recent.values, 22)
		})
	})
	t.Run("NextAndState", func(t *testing.T) {
		type testStep struct {
			value  float64
			closed bool
			next   State
		}
		testCases := []struct {
			name   string
			config MachineConfig
			steps  []testStep
		}{
			{
				name: "SkipResting",
				config: MachineConfig{
					HistoryLen:    1,
					RecentLen:     1,
					AbsoluteMax:   1.0,
					ClosingMax:    1,
					ClosingStreak: 1,
				},
				steps: []testStep{
					{2.0, false, Closing},
					{2.0, true, Watching},
				},
			},
			{
				name: "SkipClosingZeroMax",
				config: MachineConfig{
					HistoryLen:    1,
					RecentLen:     1,
					AbsoluteMax:   1.0,
					ClosingStreak: 1,
					RestingMax:    1,
				},
				steps: []testStep{
					{1.0, true, Resting},
					{0.5, true, Watching},
				},
			},
			{
				name: "SkipClosingZeroStreak",
				config: MachineConfig{
					HistoryLen:  1,
					RecentLen:   1,
					AbsoluteMax: 3.0,
					PercentMax:  1.0,
					ClosingMax:  1,
					RestingMax:  1,
				},
				steps: []testStep{
					{1.0, true, Watching},
					{1.0, true, Watching},
					{2.0, true, Resting},
					{2.0, true, Watching},
				},
			},
			{
				name: "Absolute",
				config: MachineConfig{
					HistoryLen:    1,
					RecentLen:     2,
					AbsoluteMax:   10.0,
					ClosingStreak: 2,
					ClosingMax:    3,
					RestingMax:    2,
				},
				steps: []testStep{
					{10.0, false, Closing},
					{20.0, false, Closing},
					{10.0, true, Closing},
					{10.0, false, Closing},
					{5.0, false, Closing},
					{5.0, true, Closing},
					{5.0, false, Closing},
					{15.0, true, Resting},
					{15.0, false, Resting},
					{10.0, false, Closing},
					{10.0, true, Closing},
					{11.0, true, Resting},
					{10.0, false, Resting},
					{5.0, false, Watching},
					{10.0, false, Watching},
					{10.0, false, Closing},
				},
			},
			{
				name: "Percent",
				config: MachineConfig{
					HistoryLen:    1,
					RecentLen:     1,
					PercentMax:    1.0,
					ClosingStreak: 1,
					ClosingMax:    1,
					RestingMax:    2,
				},
				steps: []testStep{
					{1.0, false, Closing},
					{1.0, true, Resting},
					{10.0, false, Resting},
					{15.0, false, Watching},
					{29.5, false, Watching},
					{60.0, false, Closing},
					{1.0, true, Resting},
					{2.0, false, Resting},
					{2.0, false, Watching},
					{3.0, false, Watching},
					{6.0, false, Closing},
				},
			},
			{
				name: "BothAbsoluteAndPercent",
				config: MachineConfig{
					HistoryLen:    5,
					RecentLen:     1,
					AbsoluteMax:   10.0,
					PercentMax:    1.0,
					ClosingStreak: 2,
					ClosingMax:    3,
					RestingMax:    1,
				},
				steps: []testStep{
					{5.0, false, Watching},
					{5.0, false, Watching},
					{20.0, false, Closing},
					{5.0, true, Closing},
					{5.0, true, Resting},
					{0.0, false, Watching},
					{0.0, false, Watching},
					{0.0, false, Watching},
					{5.0, false, Closing},
					{1.0, false, Closing},
					{1.0, true, Closing},
					{1.0, false, Closing},
					{1.0, true, Closing},
					{1.0, false, Closing},
					{1.0, true, Resting},
					{2.0, true, Closing},
					{2.0, true, Closing},
					{2.0, true, Resting},
					{2.0, false, Watching},
					{2.0, false, Watching},
					{4.0, false, Closing},
					{4.0, true, Closing},
					{4.0, true, Resting},
					{4.0, true, Watching},
					{8.0, false, Closing},
					{8.0, true, Closing},
					{8.0, true, Resting},
					{8.0, false, Watching},
					{8.0, false, Watching},
					{9.0, false, Watching},
					{9.25, false, Watching},
					{9.50, false, Watching},
					{9.75, false, Watching},
					{10.0, false, Closing},
				},
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				m := NewMachine(testCase.config)
				state := Watching
				for i, step := range testCase.steps {
					t.Run(strconv.Itoa(i), func(t *testing.T) {
						assert.Equal(t, state, m.State())
						next, prev := m.Next(step.value, step.closed)
						assert.Equal(t, state, prev)
						assert.Equal(t, step.next, next)
						state = next
					})
				}
			})
		}
	})
}

type mockMachine struct {
	mock.Mock
}

func newMockMachine(t *testing.T) *mockMachine {
	m := &mockMachine{}
	m.Test(t)
	return m
}

func (m *mockMachine) State() State {
	args := m.Called()
	return args.Get(0).(State)
}

func (m *mockMachine) Next(value float64, closed bool) (next State, prev State) {
	args := m.Called(value, closed)
	next, prev = args.Get(0).(State), args.Get(1).(State)
	return
}
