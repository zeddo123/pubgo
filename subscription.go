package pubgo

import (
	"context"
	"fmt"
	"time"
)

type MsgHandler func(msg any) error
type optHandler func(cfg *subscriptionOps)

type subscriptionOps struct {
	// Size of a subscriber's msg buffer. If buffer is full, new msg are dropped.
	BufferSize int
	// Timeout after which the subscriber return without waiting for a message.
	ReadTimeout time.Duration
}

type Subscription struct {
	id            int
	bufferSize    int
	readTimeout   time.Duration
	msgs          chan any
	done          chan<- int
	doneListening chan struct{}
}

func defaultSubscriptionOps() subscriptionOps {
	return subscriptionOps{
		BufferSize:  0,
		ReadTimeout: time.Second * 10,
	}
}

func WithReadTimeout(duration time.Duration) optHandler {
	return func(cfg *subscriptionOps) {
		cfg.ReadTimeout = duration
	}
}

func WithBufferSize(s int) optHandler {
	return func(cfg *subscriptionOps) {
		cfg.BufferSize = s
	}
}

// Do runs fn each time a msg that is received. Call blocks until an fn call returns an error.
// Function could block undefinetely if subscribtion never gets canceled.
func (s *Subscription) Do(ctx context.Context, fn MsgHandler) error {
	for msg := range s.msgs {
		err := fn(msg)
		if err != nil {
			return err
		}
	}

	return nil
}

// NextWithTimeout reads the next available msg or returns a timeout error if
// no msg is available.
func (s *Subscription) NextWithTimeout(ctx context.Context) (any, error) {
	select {
	case msg, ok := <-s.msgs:
		if !ok {
			return msg, fmt.Errorf("subscription closed: could not read msgs")
		}

		return msg, nil
	case <-time.After(s.readTimeout):
		return nil, fmt.Errorf("readtimeout: could read msg in time")
	}
}

// Next blocks until a msg is received. Returns an error if performed on
// a closed subscription.
func (s *Subscription) Next(ctx context.Context) (any, error) {
	msg, ok := <-s.msgs
	if !ok {
		return msg, fmt.Errorf("subscribtion closed: could not read msgs")
	}

	return msg, nil
}

// Done notifies the Bus to stop publishing msgs to it. Some publishing
// strategies assume well-behaving subscribers. Without this call, publishers
// could become blocked until msgs are consummed.
func (s *Subscription) Done() {
	close(s.doneListening)
	s.done <- s.id
}

func (s *Subscription) ID() int {
	return s.id
}
