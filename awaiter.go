package jooki

import (
	"errors"
	"fmt"
	"time"
)

type Awaiter struct {
	c *Client
	chid int
	ch chan *StateUpdate
	update *StateUpdate
}

func NewAwaiter(client *Client, id int, ch chan *StateUpdate) *Awaiter {
	state := client.GetState()
	a := &Awaiter{
		c: client,
		chid: id,
		ch: ch,
		update: &StateUpdate{
			Before: state,
			After: state,
			Deltas: []*JookiState{},
		},
	}
	return a
}

func (a *Awaiter) GetChannel() chan *StateUpdate {
	return a.ch
}

func (a *Awaiter) GetState() *JookiState {
	return a.update.After
}

func (a *Awaiter) GetInitialState() *JookiState {
	return a.update.Before
}

func (a *Awaiter) GetDeltas() []*JookiState {
	return a.update.Deltas
}

func (a *Awaiter) GetDeltaCount() int {
	return len(a.update.Deltas)
}

func (a *Awaiter) Read(timer *time.Timer) (*StateUpdate, bool) {
	if a.ch == nil {
		return a.update, false
	}
	select {
	case update := <-a.ch:
		a.update.After = update.After
		a.update.Deltas = append(a.update.Deltas, update.Deltas...)
		return a.update, true
	case <-timer.C:
		return a.update, false
	}
}

func (a *Awaiter) Write(update *StateUpdate) error {
	if len(a.ch) < cap(a.ch) {
		a.ch <- update
		return nil
	}
	return fmt.Errorf("awaiter %d full", a.chid)
}

func (a *Awaiter) Close() *StateUpdate {
	if a.ch == nil {
		return a.update
	}
	close(a.ch)
	ch := a.ch
	a.ch = nil
	a.c.RemoveAwaiter(a.chid)
	for len(a.ch) > 0 {
		update := <-ch
		a.update.After = update.After
		a.update.Deltas = append(a.update.Deltas, update.Deltas...)
	}
	return a.update
}

func (a *Awaiter) Closed() bool {
	return a.ch == nil
}

func (a *Awaiter) WaitFor(f func(state *JookiState) bool, timeout time.Duration) (*JookiState, error) {
	state := a.GetState()
	if f(state) {
		return state, nil
	}
	timer := time.NewTimer(timeout)
	i := 0
	for {
		update, ok := a.Read(timer)
		if !ok {
			return update.After, errors.New("timeout")
		}
		ok = false
		for i < len(update.Deltas) {
			if f(update.Deltas[i]) {
				ok = true
				break
			}
			i += 1
		}
		if ok || f(update.After) {
			state = update.After
			break
		}
	}
	if !timer.Stop() {
		<-timer.C
	}
	return state, nil
}

