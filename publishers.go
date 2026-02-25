package pubgo

import (
	"log"
	"sync"
	"time"
)

type Publisher func(b *Bus, subs []*Subscription, msg any) error

func GaranteedPublish() Publisher {
	return func(b *Bus, subs []*Subscription, msg any) error {
		return garanteedPublish(b, subs, msg)
	}
}

func garanteedPublish(b *Bus, subs []*Subscription, msg any) error {
	for _, sub := range subs {
		if sub != nil {
			b.acquireSub(sub, func(sub *Subscription) {
				select {
				case <-sub.doneListening:
				case sub.msgs <- msg:
				}
			})
		}
	}

	return nil
}

func AvailablePublish() Publisher {
	return func(b *Bus, subs []*Subscription, msg any) error {
		return availablePublish(b, subs, msg)
	}
}

func availablePublish(b *Bus, subs []*Subscription, msg any) error {
	for _, sub := range subs {
		if sub != nil {
			b.acquireSub(sub, func(sub *Subscription) {
				select {
				case <-sub.doneListening:
				case sub.msgs <- msg:
				default:
				}
			})
		}
	}

	return nil
}

func NonBlockingPublish() Publisher {
	return func(b *Bus, subs []*Subscription, msg any) error {
		return nonBlockingPublish(b, subs, msg, time.Hour)
	}
}

func NonBlockingPublishWithTimeout(timeout time.Duration) Publisher {
	return func(b *Bus, subs []*Subscription, msg any) error {
		return nonBlockingPublish(b, subs, msg, timeout)
	}
}

func nonBlockingPublish(b *Bus, subs []*Subscription, msg any, timeout time.Duration) error {
	var wg sync.WaitGroup

	for _, sub := range subs {
		select {
		case <-sub.doneListening:
			continue
		default:
		}

		wg.Go(func() {
			b.acquireSub(sub, func(sub *Subscription) {
				select {
				case <-sub.doneListening:
				case sub.msgs <- msg:
				case <-time.After(timeout):
					log.Printf("dropping msg: broadcast timeout for subscriber %d", sub.id)
				}
			})
		})
	}

	return nil
}
