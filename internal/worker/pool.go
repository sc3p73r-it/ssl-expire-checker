package worker

import (
	"sync"
)

// Job is a unit of work executed by the pool.
type Job[T any] struct {
	Payload T
}

// Pool runs jobs concurrently with a fixed number of workers.
type Pool[T any] struct {
	workers int
}

// NewPool creates a worker pool with the given worker count.
func NewPool[T any](workers int) *Pool[T] {
	if workers < 1 {
		workers = 1
	}
	return &Pool[T]{workers: workers}
}

// Run processes all jobs; handler is called once per job (possibly concurrent).
func (p *Pool[T]) Run(jobs []Job[T], handler func(T)) {
	if len(jobs) == 0 {
		return
	}
	ch := make(chan Job[T], len(jobs))
	var wg sync.WaitGroup
	for i := 0; i < p.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range ch {
				handler(j.Payload)
			}
		}()
	}
	for _, j := range jobs {
		ch <- j
	}
	close(ch)
	wg.Wait()
}
