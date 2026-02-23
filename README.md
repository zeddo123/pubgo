# pubgo
lightweight in-memory pub sub process for golang with multiple publishing and consuming strategies.

```golang
ctx, cancel := context.WithCancel(context.Background())
bus := pubgo.NewBusWithContext(ctx, pubgo.BusOps{
    InitialTopicsCap: 10,
    InitialSubsCap:   10,
    PublishStrat:     pubgo.NonBlockingPublish(),
})

// Start consuming msgs from the topics
go func() {
    // The subscriber selectes the size of its msgs buffer. Depending
    // on which publishing strategy is chosen, when the buffer is full
    // the publisher might block until a msg is pulled or the msgs are dropped.
    s := b.Subscribe("topic#1", pubgo.WithBufferSize(100))
    defer s.Done()

    // Next blocks until a msg is received or the subscription has been
    // cancelled.
    msg, err := s.Next(context.Background())
    
    // NextWithTimeout blocks until a msg is received or a timeout is reached
    msg, err := s.NextWithTimeout(context.Background())

    // Do some work.
}()

err := bus.Publish("topic#1", "some msg to queue")

```
