package pubgo

import (
	"context"
	"errors"
	"sync"
)

var NoAvailableSubs = errors.New("no available subscribers")

type Bus struct {
	l           sync.RWMutex
	subSync     map[int]*sync.WaitGroup
	topics      map[string]map[int]struct{}
	subs        map[int]*Subscription
	cancelSub   chan int
	cancelTopic chan string
	publisher   publisher
	nextID      int
}

type BusOps struct {
	InitialTopicsCap int
	InitialSubsCap   int
	PublishStrat     publisher
}

func DefaultOps() BusOps {
	return BusOps{
		InitialTopicsCap: 50,
		InitialSubsCap:   100,
		PublishStrat:     GaranteedPublish(),
	}
}

func NewBus(ops BusOps) *Bus {
	b := newBus(ops)

	go b.watch(context.Background())

	return b
}

func NewBusWithContext(ctx context.Context, ops BusOps) *Bus {
	b := newBus(ops)

	go b.watch(ctx)

	return b
}

func newBus(ops BusOps) *Bus {
	return &Bus{
		l:           sync.RWMutex{},
		topics:      make(map[string]map[int]struct{}, ops.InitialTopicsCap),
		subs:        make(map[int]*Subscription, ops.InitialSubsCap),
		subSync:     make(map[int]*sync.WaitGroup),
		cancelSub:   make(chan int),
		cancelTopic: make(chan string),
		publisher:   ops.PublishStrat,
	}
}

func (b *Bus) watch(ctx context.Context) {
	for {
		select {
		case id := <-b.cancelSub:
			b.removeSub(id)
		case id := <-b.cancelTopic:
			b.removeTopic(id)
		case <-ctx.Done():
			close(b.cancelSub)
			close(b.cancelTopic)
			return
		}
	}
}

func (b *Bus) removeTopic(id string) {
	b.l.Lock()
	defer b.l.Unlock()

	for subID := range b.topics[id] {
		if s, ok := b.subs[subID]; ok {
			close(s.msgs)
			delete(b.subs, subID)
		}
	}

	delete(b.topics, id)
}

func (b *Bus) writeSub(s *Subscription, topics ...string) int {
	b.l.Lock()
	defer b.l.Unlock()

	s.id = b.nextID
	b.nextID++

	b.subs[s.id] = s

	for _, topic := range topics {
		if _, ok := b.topics[topic]; !ok {
			b.topics[topic] = make(map[int]struct{})
		}
		b.topics[topic][s.id] = struct{}{}
	}

	b.subSync[s.id] = &sync.WaitGroup{}

	return s.id
}

// send stuff logic goes here. Notifies "cleaner" go routine to safely close
// sub's channel
func (b *Bus) acquireSub(s *Subscription, fn func(s *Subscription)) {
	b.l.RLock()
	wg := b.subSync[s.id]
	b.l.RUnlock()

	wg.Add(1)
	fn(s)
	wg.Done()
}

func (b *Bus) getSubs(topic string) ([]*Subscription, error) {
	b.l.RLock()
	defer b.l.RUnlock()

	if m, ok := b.topics[topic]; ok {
		s := make([]*Subscription, 0, len(m))
		for x := range m {
			if sub, ok := b.subs[x]; ok {
				s = append(s, sub)
			} else {
				// remove sub from topic
				go b.removeSubFromTopic(topic, x)
			}
		}

		return s, nil
	}

	return nil, NoAvailableSubs
}

func (b *Bus) removeSub(id int) {
	b.l.Lock()
	defer b.l.Unlock()

	s, ok := b.subs[id]
	if !ok {
		return
	}

	delete(b.subs, id)

	// multiple senders could be still sending to
	// the sub's channel. We must wait until all have
	// received the done signal before closing the channel
	b.subSync[id].Wait()

	close(s.msgs)
}

func (b *Bus) removeSubFromTopic(topic string, id int) {
	b.l.Lock()
	defer b.l.Unlock()

	delete(b.topics[topic], id)
}

func (b *Bus) Subscribe(topic string, opts ...optHandler) *Subscription {
	opt := defaultSubscriptionOps()
	for _, fn := range opts {
		fn(&opt)
	}

	s := &Subscription{
		bufferSize:    opt.BufferSize,
		readTimeout:   opt.ReadTimeout,
		msgs:          make(chan any, opt.BufferSize),
		done:          b.cancelSub,
		doneListening: make(chan struct{}),
	}

	_ = b.writeSub(s, topic)

	return s
}

func (b *Bus) Publish(topic string, msg any) error {
	subs, err := b.getSubs(topic)
	if err != nil {
		return err
	}

	return b.publisher(b, subs, msg)
}
