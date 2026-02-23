package pubgo_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/zeddo123/pubgo"
)

func TestPublisher(t *testing.T) {
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())

	b := pubgo.NewBusWithContext(ctx, pubgo.DefaultOps())

	t.Log("created subscriber")
	s := b.Subscribe("test", pubgo.WithBufferSize(0))

	wg.Add(1)
	go func() {
		defer wg.Done()
		t.Log("about to read 1")
		msg, err := s.Next(context.Background())
		t.Log("got message 2", msg)
		t.Log(err)

		t.Log("about to read 2")
		msg, err = s.Next(context.Background())
		t.Log("got message 2", msg)
		t.Log(err)
	}()

	time.Sleep(2 * time.Second)

	t.Log("about to publish")
	err := b.Publish("test", "msg!")
	t.Log(err)

	s.Done()
	cancel()

	wg.Wait()
}

func BenchmarkPublish(b *testing.B) {
	msgs := 100
	subscribers := 3
	publishers := 1

	subs := make([]*pubgo.Subscription, subscribers)

	ctx, cancel := context.WithCancel(context.Background())
	bus := pubgo.NewBusWithContext(ctx, pubgo.BusOps{
		InitialTopicsCap: 10,
		InitialSubsCap:   10,
		PublishStrat:     pubgo.GaranteedPublish(),
	})

	for b.Loop() {
		var wgs sync.WaitGroup
		var wgp sync.WaitGroup

		for i := range subscribers {
			subs[i] = bus.Subscribe("pub msg", pubgo.WithBufferSize(0))
		}

		for i := range subscribers {
			wgs.Go(func() {
				s := subs[i]
				defer s.Done()

				for range msgs * publishers {
					_, err := s.Next(context.Background())
					if err != nil {
						b.Log("could not read msg", i, err)
						b.Fail()
						return
					}
				}
			})
		}

		for i := range publishers {
			wgp.Go(func() {
				for msg := range msgs {
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
