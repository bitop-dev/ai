package ai

import (
	"context"
	"sync/atomic"
)

// Stream provides an iterator-style wrapper over a channel.
type Stream[T any] struct {
	ctx    context.Context
	cancel context.CancelFunc
	ch     <-chan T
	value  T
	err    error
	closed atomic.Bool
}

func newStream[T any](ctx context.Context, cancel context.CancelFunc, ch <-chan T) *Stream[T] {
	if ctx == nil {
		ctx = context.Background()
	}
	if ch == nil {
		empty := make(chan T)
		close(empty)
		ch = empty
	}
	return &Stream[T]{
		ctx:    ctx,
		cancel: cancel,
		ch:     ch,
	}
}

// Next advances the iterator and reports whether a value is available.
func (s *Stream[T]) Next() bool {
	if s.closed.Load() {
		return false
	}
	select {
	case <-s.ctx.Done():
		if s.err == nil {
			s.err = s.ctx.Err()
		}
		s.closed.Store(true)
		return false
	case value, ok := <-s.ch:
		if !ok {
			s.closed.Store(true)
			return false
		}
		s.value = value
		return true
	}
}

// Value returns the current value after Next reports true.
func (s *Stream[T]) Value() T {
	return s.value
}

// Err returns the terminal error, if any.
func (s *Stream[T]) Err() error {
	return s.err
}

// Close cancels the stream and makes Next return false.
func (s *Stream[T]) Close() {
	s.closed.Store(true)
	if s.cancel != nil {
		s.cancel()
	}
}
