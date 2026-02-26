package pubgo_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeddo123/pubgo"
)

type benchmarkConfig struct {
	subs             int
	pubs             int
	msgsPerPublisher int
	subWorkDuration  time.Duration
	pubWorkDuration  time.Duration
	strat            pubgo.Publisher //nolint: misspell
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

	l := &sync.Mutex{}
	totalDelay := 0 * time.Second
	avgDelay := make([]time.Duration, 0, cfg.subs*b.N)

	for b.Loop() {
		var wgs sync.WaitGroup

		var wgp sync.WaitGroup

		for i := range cfg.subs {
			subs[i] = bus.Subscribe("pub msg", pubgo.WithBufferSize(10))
		}

		for i := range cfg.subs {
			wgs.Go(func() {
				subTotalDelay := 0 * time.Second
				delays := make([]time.Duration, cfg.msgsPerPublisher*cfg.pubs)

				s := subs[i]
				defer s.Done()

				for j := range cfg.msgsPerPublisher * cfg.pubs {
					// do some work
					time.Sleep(cfg.subWorkDuration)

					msg, err := s.Next(context.Background())
					received := time.Now()
					if err != nil {
						b.Log("could not read msg", i, err)
						b.Fail()

						return
					}

					t, ok := msg.(time.Time)
					require.True(b, ok)

					delay := received.Sub(t)

					subTotalDelay += delay
					delays[j] = delay
				}

				l.Lock()
				totalDelay += subTotalDelay
				avgDelay = append(avgDelay, averageDuration(delays))
				l.Unlock()
			})
		}

		for range cfg.pubs {
			wgp.Go(func() {
				for range cfg.msgsPerPublisher {
					// Do work.
					time.Sleep(cfg.pubWorkDuration)

					err := bus.Publish("pub msg", time.Now())
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

	b.ReportMetric(float64(totalDelay)/float64(b.N), "total_delay_ns/op")
	b.ReportMetric(float64(averageDuration(avgDelay))/float64(b.N), "avg_delay_ns/op")
}

func averageDuration(durs []time.Duration) time.Duration {
	if len(durs) == 0 {
		return 0
	}

	var avg time.Duration
	for i, d := range durs {
		avg += (d - avg) / time.Duration(i+1)
	}

	return avg
}
