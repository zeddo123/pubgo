[![Build](https://github.com/zeddo123/pubgo/actions/workflows/build.yaml/badge.svg)](https://github.com/zeddo123/pubgo/actions/workflows/build.yaml)
[![golangci-lint](https://github.com/zeddo123/pubgo/actions/workflows/lint.yaml/badge.svg)](https://github.com/zeddo123/pubgo/actions/workflows/lint.yaml)

# pubgo

A lightweight, in-process pub/sub library for Go. `pubgo` runs entirely in
memory inside a single process — there's no broker, no network hop, and no
external dependency to stand up. It's meant for fanning messages out to
multiple in-process consumers (event buses, decoupling internal components,
test fixtures, etc.), not for durable or cross-process messaging.

## Scope

What `pubgo` gives you:
- A topic-based bus: subscribers register interest in a topic string, publishers
  send messages to a topic, and every current subscriber of that topic gets a copy.
- A choice of **publishing strategies** trading off delivery guarantees against
  blocking behavior (see below).
- Per-subscriber buffering and read timeouts, configurable independently for
  each `Subscribe` call.
- Combining multiple subscriptions into a single consumable stream (fan-in).

What it deliberately does **not** do:
- Persist messages to disk or any external store — everything lives in memory
  and disappears when the process exits.
- Cross-process or networked delivery — publishers and subscribers must live
  in the same Go process.
- Message ordering guarantees across topics, or replay/history of past messages.

## Installation
```sh
go get github.com/zeddo123/pubgo
```

## Concepts

- **Bus**: the central dispatcher. Owns all topics and subscriptions and
  applies the configured publishing strategy on every `Publish` call.
- **Subscription**: returned by `Bus.Subscribe`, represents one consumer's
  interest in a topic. Messages arrive on `Subscription.Msgs`, or can be read
  via `Next`, `NextWithTimeout`, or `Do`.
- **Publisher (strategy)**: the function that decides how a published message
  is delivered to the topic's current subscribers.

## Creating a bus

```go
// NewBus starts an internal background goroutine tied to context.Background().
bus := pubgo.NewBus(pubgo.DefaultOps())

// NewBusWithContext ties that goroutine's lifetime to a context you control.
// Cancelling ctx stops the bus's cleanup goroutine and releases its resources.
ctx, cancel := context.WithCancel(context.Background())
bus := pubgo.NewBusWithContext(ctx, pubgo.BusOps{
    InitialTopicsCap: 10, // preallocate capacity for this many topics
    InitialSubsCap:   10, // preallocate capacity for this many subscriptions
    PublishStrat:     pubgo.NonBlockingPublish(),
})
defer cancel()
```

`pubgo.DefaultOps()` returns sane defaults (`GaranteedPublish` as the strategy)
if you don't need to tune capacities or pick a different strategy.

## Publishing strategies

Set via `BusOps.PublishStrat`, applied on every `bus.Publish` call:

| Strategy | Behavior |
|---|---|
| `pubgo.GaranteedPublish()` | Blocks until **every** subscriber has received the message. Guarantees delivery, but a slow or stalled subscriber can block the publisher indefinitely. |
| `pubgo.AvailablePublish()` | Sends to subscribers that are immediately ready to receive; subscribers whose buffer is full are skipped for that message. Never blocks, but delivery isn't guaranteed. |
| `pubgo.NonBlockingPublish()` | Spawns a goroutine per subscriber and sends with a 1-hour timeout, so `Publish` itself returns immediately. |
| `pubgo.NonBlockingPublishWithTimeout(d)` | Same as `NonBlockingPublish`, but with a configurable per-subscriber send timeout `d` after which the message is dropped and logged. |

Choose `GaranteedPublish` when subscribers are well-behaved and must not miss
messages; `AvailablePublish` or `NonBlockingPublish`/`NonBlockingPublishWithTimeout`
when the publisher must never be slowed down by a stuck subscriber.

## Subscribing

```go
s := bus.Subscribe("topic#1",
    // Size of the subscriber's message buffer (default: 0, unbuffered).
    // Once full, publishers may block or drop messages depending on the
    // publishing strategy in use.
    pubgo.WithBufferSize(100),

    // Timeout used by Next/NextWithTimeout when waiting for a message
    // (default: 10s).
    pubgo.WithReadTimeout(5*time.Second),
)
defer s.Done() // stops the bus from publishing to this subscription and releases it

s.ID() // unique subscription id, useful for logging/debugging
```

### Reading messages

```go
// Next blocks until a message arrives or the subscription is closed.
msg, err := s.Next(context.Background())

// NextWithTimeout blocks until a message arrives or s's read timeout elapses.
msg, err := s.NextWithTimeout(context.Background())

// Or read the channel directly.
for msg := range s.Msgs {
    // ...
}
```

### Consuming with a callback

```go
err := s.Do(ctx, func(topic string, msg any) error {
    log.Println("received msg on", topic, ":", msg)

    str, ok := msg.(string)
    if !ok {
        return fmt.Errorf("could not read msg")
    }

    if str == "done" {
        s.Done()
        return fmt.Errorf("done reading")
    }

    return nil
})
```

`Do` runs `fn` for every message received and blocks until `fn` returns a
non-nil error or the subscription is closed.

### Combining subscriptions (fan-in)

Multiple subscriptions — even across different topics — can be merged into a
single stream, useful when a consumer cares about several topics but wants to
process them through one loop:

```go
s1 := bus.Subscribe("topic#1")
s2 := bus.Subscribe("topic#2")

combined := pubgo.Combine(100, s1, s2) // 100 = output channel buffer size
defer combined.Done()                  // closes s1 and s2 too

topic, msg, err := combined.Next(context.Background())
// combined.NextWithTimeout(ctx, timeout) and combined.Do(ctx, fn) are also available
```

## Publishing

```go
err := bus.Publish("topic#1", "some msg to queue")
```

### Publishing without subscribers

Publishing to a topic with no active subscribers returns `pubgo.ErrNoAvailableSubs`:

```go
err := bus.Publish("topic#42", "important message")
if errors.Is(err, pubgo.ErrNoAvailableSubs) {
    // handle no subscribers
} else if err != nil {
    // handle other errors
}
```

## Full example

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

bus := pubgo.NewBusWithContext(ctx, pubgo.BusOps{
    InitialTopicsCap: 10,
    InitialSubsCap:   10,
    PublishStrat:     pubgo.NonBlockingPublish(),
})

go func() {
    s := bus.Subscribe("topic#1", pubgo.WithBufferSize(100))
    defer s.Done()

    for {
        msg, err := s.Next(context.Background())
        if err != nil {
            return
        }
        // do some work with msg
        _ = msg
    }
}()

err := bus.Publish("topic#1", "some msg to queue")
```

### Benchmark
```sh
goos: linux
goarch: amd64
pkg: github.com/zeddo123/pubgo
cpu: 13th Gen Intel(R) Core(TM) i9-13950HX
                             │  bench.txt   │
                             │    sec/op    │
NonBlockingPublish_50_3_3-32   56.64m ± ∞ ¹
GuaranteedPublish_50_3_3-32    55.78m ± ∞ ¹
geomean                        56.21m
¹ need >= 6 samples for confidence interval at level 0.95

                             │   bench.txt   │
                             │     B/op      │
NonBlockingPublish_50_3_3-32   153.2Ki ± ∞ ¹
GuaranteedPublish_50_3_3-32    9.041Ki ± ∞ ¹
geomean                        37.21Ki
¹ need >= 6 samples for confidence interval at level 0.95

                             │  bench.txt   │
                             │  allocs/op   │
NonBlockingPublish_50_3_3-32   2.893k ± ∞ ¹
GuaranteedPublish_50_3_3-32     490.0 ± ∞ ¹
geomean                        1.191k
¹ need >= 6 samples for confidence interval at level 0.95
```
