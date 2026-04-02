package scheduler

import (
	"context"
	"crypto/rand"
	"math/big"
	"sync"
	"time"
)

type CollectFunc func(ctx context.Context)

type probeSchedule struct {
	name    string
	min     time.Duration
	max     time.Duration
	collect CollectFunc
}

type Scheduler struct {
	mu      sync.Mutex
	probes  []probeSchedule
	wg      sync.WaitGroup
}

func New() *Scheduler {
	return &Scheduler{}
}

func (s *Scheduler) Add(name string, min, max time.Duration, fn CollectFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.probes = append(s.probes, probeSchedule{
		name:    name,
		min:     min,
		max:     max,
		collect: fn,
	})
}

func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.probes {
		s.wg.Add(1)
		go s.run(ctx, p)
	}
}

func (s *Scheduler) Wait() {
	s.wg.Wait()
}

func (s *Scheduler) run(ctx context.Context, p probeSchedule) {
	defer s.wg.Done()

	for {
		interval, err := randomInterval(p.min, p.max)
		if err != nil {
			interval = p.min
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
			p.collect(ctx)
		}
	}
}

func randomInterval(min, max time.Duration) (time.Duration, error) {
	diff := max - min
	if diff <= 0 {
		return min, nil
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(diff)))
	if err != nil {
		return 0, err
	}
	return min + time.Duration(n.Int64()), nil
}
