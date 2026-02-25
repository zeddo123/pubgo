package pubgo_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/zeddo123/pubgo"
)

type benchmarkConfig struct {
	subs             int
	pubs             int
	msgsPerPublisher int
	subWorkDuration  time.Duration
	pubWorkDuration  time.Duration
	strat            pubgo.Publisher
}

func TestPublisher(t *testing.T) {
	var wg sync.WaitGroup

	ctx, cancel := context.WithCancel(context.Background())

	b := pubgo.NewBusWithContext(ctx, pubgo.DefaultOps())

	t.Log("created subscriber")

	s := b.Subscribe("test", pubgo.WithBufferSize(0))

	wg.Go(func() {
		t.Log("about to read 1")

		msg, err := s.Next(context.Background())
		t.Log("got message 2", msg)
		t.Log(err)

		t.Log("about to read 2")

		msg, err = s.Next(context.Background())
		t.Log("got message 2", msg)
		t.Log(err)
	})

	time.Sleep(2 * time.Second)

	t.Log("about to publish")

	err := b.Publish("test", "msg!")
	t.Log(err)

	s.Done()
	cancel()

	wg.Wait()
}

// BenchmarkNonBlockingPublish_50_3_3 benchmarks the NonBlocking publishing strategy
// with 50 msgs per publishers, 3 subscribers with a 50 Millisecond work delay, and 3 publishers with 1 millisecond.
func BenchmarkNonBlockingPublish_50_3_3(b *testing.B) {
	benchmarkPublishingStrats(b, benchmarkConfig{
		msgsPerPublisher: 50,
		subs:             3,
		pubs:             3,
		subWorkDuration:  time.Millisecond * 50,
		pubWorkDuration:  time.Millisecond * 1,
		strat:            pubgo.NonBlockingPublish(), //nolint: misspell
	})
}

// BenchmarkNonBlockingPublish_50_3_3 benchmarks the Guaranteed publishing strategy
// with 50 msgs per publishers, 3 subscribers with a 50 Millisecond work delay, and 3 publishers with 1 millisecond.
func BenchmarkGuaranteedPublish_50_3_3(b *testing.B) {
	benchmarkPublishingStrats(b, benchmarkConfig{
		msgsPerPublisher: 50,
		subs:             3,
		pubs:             3,
		subWorkDuration:  time.Millisecond * 50,
		pubWorkDuration:  time.Millisecond * 1,
		strat:            pubgo.GaranteedPublish(), //nolint: misspell
	})
}

func benchmarkPublishingStrats(b *testing.B, cfg benchmarkConfig) {
	subs := make([]*pubgo.Subscription, cfg.subs)

	ctx, cancel := context.WithCancel(context.Background())
	bus := pubgo.NewBusWithContext(ctx, pubgo.BusOps{
		InitialTopicsCap: 10,
		InitialSubsCap:   10,
		PublishStrat:     cfg.strat,
	})

	for b.Loop() {
		var wgs sync.WaitGroup

		var wgp sync.WaitGroup

		for i := range cfg.subs {
			subs[i] = bus.Subscribe("pub msg", pubgo.WithBufferSize(0))
		}

		for i := range cfg.subs {
			wgs.Go(func() {
				s := subs[i]
				defer s.Done()

				for range cfg.msgsPerPublisher * cfg.pubs {
					_, err := s.Next(context.Background())
					if err != nil {
						b.Log("could not read msg", i, err)
						b.Fail()

						return
					}
				}
			})
		}

		for i := range cfg.pubs {
			wgp.Go(func() {
				for msg := range cfg.msgsPerPublisher {
					m := fmt.Sprintf("%d_%d", i, msg)

					// Do work.
					time.Sleep(time.Millisecond * 1)

					err := bus.Publish("pub msg", m)
					if err != nil {
						b.Fail()

						return
					}
				}
			})
		}

		// wait for publishers to finish
		wgp.Wait()
		// wait for subscribers to finish
		wgs.Wait()
	}

	cancel()
}
