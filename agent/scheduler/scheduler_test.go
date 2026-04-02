package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRandomInterval(t *testing.T) {
	min := time.Second
	max := 5 * time.Second

	for i := 0; i < 100; i++ {
		d, err := randomInterval(min, max)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, d, min, "interval should be >= min")
		assert.Less(t, d, max, "interval should be < max")
	}
}

func TestRandomInterval_MinEqualsMax(t *testing.T) {
	d, err := randomInterval(time.Second, time.Second+time.Nanosecond)
	require.NoError(t, err)
	assert.Equal(t, time.Second, d)
}

func TestScheduler_RunsCollectFunc(t *testing.T) {
	var count atomic.Int32
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	s := New()
	s.Add("test", 50*time.Millisecond, 100*time.Millisecond, func(ctx context.Context) {
		count.Add(1)
	})
	s.Start(ctx)

	time.Sleep(500 * time.Millisecond)
	cancel()
	s.Wait()

	assert.Greater(t, count.Load(), int32(2), "should have run at least a few times")
}

func TestScheduler_StopsOnCancel(t *testing.T) {
	var count atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())

	s := New()
	s.Add("test", 10*time.Millisecond, 20*time.Millisecond, func(ctx context.Context) {
		count.Add(1)
	})
	s.Start(ctx)

	time.Sleep(100 * time.Millisecond)
	cancel()
	s.Wait()

	afterCancel := count.Load()
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, afterCancel, count.Load(), "should not run after cancel")
}

func TestScheduler_MultipleProbes(t *testing.T) {
	var countA, countB atomic.Int32
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	s := New()
	s.Add("probe-a", 30*time.Millisecond, 60*time.Millisecond, func(ctx context.Context) {
		countA.Add(1)
	})
	s.Add("probe-b", 30*time.Millisecond, 60*time.Millisecond, func(ctx context.Context) {
		countB.Add(1)
	})
	s.Start(ctx)

	time.Sleep(500 * time.Millisecond)
	cancel()
	s.Wait()

	assert.Greater(t, countA.Load(), int32(0), "probe-a should have run")
	assert.Greater(t, countB.Load(), int32(0), "probe-b should have run")
}
