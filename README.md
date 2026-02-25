# pubgo
lightweight in-memory pub sub process for golang with multiple publishing and consuming strategies.

## Installation
```sh
go get github.com/zeddo123/pubgo
```

## Usage

```go
ctx, cancel := context.WithCancel(context.Background())
bus := pubgo.NewBusWithContext(ctx, pubgo.BusOps{
    InitialTopicsCap: 10,
    InitialSubsCap:   10,
    // pubgo provides 3 publishing strategies:
    //
    // NonBlockingPublish: spawns goroutines for each subscriber, and sends the message with a timeout.
    //
    // GaranteedPublish: blocks until all subscribers have received the msg. 
    // could block forever if subscriber are not consuming msgs.
    //
    // AvailablePublish: sends msg to available subscribers. Some subscribers might not receive the msg.
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

// you can also consume msgs through the subscription's Do member function:
s := b.Subscribe("topic#1")

// the func passed as argument to Do will run whenever a msg is received. Do exits
// when subscription has been closed (s.Done()) or when the callback function return an error.
s.Do(ctx, function(topic string, msg any) error {
    log.Println("received msg", msg)

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

err := bus.Publish("topic#1", "some msg to queue")

// run cancel to stop the bus's internal goroutine to release resources
cancel()
```
