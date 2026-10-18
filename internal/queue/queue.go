// Package queue is the only type the UI and the Server share.
// The Server publishes State. The UI sends Action.
// Actions stay in order. Only the latest unread State is kept.
package queue

import (
	"context"
	"errors"
	"sync"
)

// Link is whether this Companion has a stored Session.
type Link int

const (
	Pairing Link = iota + 1
	Linked
)

// State is the latest picture the Server has published.
// QRCode is set only while a code is ready to scan.
type State struct {
	Link   Link
	QRCode string
}

// Action is a command from the UI.
type Action struct{}

// UI is the end the terminal holds.
type UI interface {
	NextState(ctx context.Context) (State, error)
	Send(Action) error
}

// Server is the end the Companion holds.
type Server interface {
	NextAction(ctx context.Context) (Action, error)
	Publish(State) error
}

// ErrClosed is returned when the queue has been closed.
var ErrClosed = errors.New("queue closed")

// New returns the two ends of one in-process queue.
func New() (UI, Server) {
	q := &queue{}
	q.cond = sync.NewCond(&q.mu)
	return q, q
}

type queue struct {
	mu   sync.Mutex
	cond *sync.Cond

	state    State
	hasState bool
	actions  []Action
	closed   bool
}

func (q *queue) Publish(state State) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return ErrClosed
	}
	q.state = state
	q.hasState = true
	q.cond.Broadcast()
	return nil
}

func (q *queue) NextState(ctx context.Context) (State, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	stop := q.wakeOnCancel(ctx)
	defer stop()
	for !q.hasState && !q.closed && ctx.Err() == nil {
		q.cond.Wait()
	}
	if q.hasState {
		q.hasState = false
		return q.state, nil
	}
	if ctx.Err() != nil {
		return State{}, ctx.Err()
	}
	return State{}, ErrClosed
}

func (q *queue) Send(action Action) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return ErrClosed
	}
	q.actions = append(q.actions, action)
	q.cond.Broadcast()
	return nil
}

func (q *queue) NextAction(ctx context.Context) (Action, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	stop := q.wakeOnCancel(ctx)
	defer stop()
	for len(q.actions) == 0 && !q.closed && ctx.Err() == nil {
		q.cond.Wait()
	}
	if len(q.actions) > 0 {
		action := q.actions[0]
		q.actions = q.actions[1:]
		return action, nil
	}
	if ctx.Err() != nil {
		return Action{}, ctx.Err()
	}
	return Action{}, ErrClosed
}

// wakeOnCancel broadcasts when ctx is done so a waiter can return.
// The caller must hold q.mu, and must call the returned stop function.
func (q *queue) wakeOnCancel(ctx context.Context) func() {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			q.mu.Lock()
			q.cond.Broadcast()
			q.mu.Unlock()
		case <-done:
		}
	}()
	return func() { close(done) }
}
