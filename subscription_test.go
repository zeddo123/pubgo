package pubgo_test

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/magiconair/properties/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeddo123/pubgo"
)

func TestSubDo(t *testing.T) {
	var wg sync.WaitGroup

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	b := pubgo.NewBusWithContext(ctx, pubgo.DefaultOps())

	count := 0

	s := b.Subscribe("topic#1", pubgo.WithBufferSize(0),
		pubgo.WithReadTimeout(time.Second*1))

	wg.Go(func() {
		s.Do(context.Background(), func(_ string, msg any) error {
			t.Log("received msg", msg)
			str, ok := msg.(string)
			require.True(t, ok, "type should be string")

			count++

			if str == "done" {
				s.Done()
				return fmt.Errorf("done reading!")
			}

			return nil
		})

	})

	time.Sleep(time.Millisecond * 1)

	msgs := 10

	for msg := range msgs {
		require.NoError(t, b.Publish("topic#1", strconv.Itoa(msg)))
	}

	require.NoError(t, b.Publish("topic#1", "done"))

	wg.Wait()

	assert.Equal(t, count, msgs+1)
}
