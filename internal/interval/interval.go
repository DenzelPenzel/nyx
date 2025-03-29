package interval

import (
	"sync"
	"time"
)

// IntervalRunner repeatedly executes a given function at specified intervals
type IntervalRunner struct {
	ticker *time.Ticker
	done   chan struct{}
	wg     sync.WaitGroup
}

// NewIntervalRunner creates and starts an IntervalRunner that calls fn at each tick of the ticker.
// The function fn receives the tick time as its argument.
// The runner can be stopped by calling the Stop method.
func NewIntervalRunner(fn func(t time.Time), delay time.Duration) *IntervalRunner {
	runner := &IntervalRunner{
		ticker: time.NewTicker(delay),
		done:   make(chan struct{}),
	}

	runner.wg.Add(1)

	go func() {
		defer runner.wg.Done()
		for {
			select {
			case t, ok := <-runner.ticker.C:
				if !ok {
					return
				}
				fn(t)
			case <-runner.done:
				return
			}
		}
	}()

	return runner
}

func (r *IntervalRunner) Stop() {
	r.ticker.Stop()
	close(r.done)
	r.wg.Wait()
}
