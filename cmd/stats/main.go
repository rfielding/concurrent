package main

import (
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"time"
)

// We ASSUME that we can't return the same value twice, which prevents 0 length intervals
func mus() int64 {
	return time.Now().UnixNano() / 100
}

// A simple test to exercise the library
func schedTest(m *Reporter) {
	sharedBottleneckQueue := 16
	concurrentRequests := 512
	ch := make(chan int, sharedBottleneckQueue)
	wg := sync.WaitGroup{}
	for i := 0; i < concurrentRequests; i++ {
		wg.Add(1)
		go func() {
			for j := 0; j < 4; j++ {
				runtime.Gosched()
				start := mus()
				tasks := rand.Intn(10000)
				for l := 0; l < int(tasks); l++ {
					ch <- 5
					runtime.Gosched()
					_ = <-ch
				}
				finish := mus()
				r := float64(tasks*10000) / float64(finish-start)
				m.Do(start, finish, r, 1)
				runtime.Gosched()
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func main() {
	m := NewReporter()
	m.Begin(mus())
	schedTest(m)
	m.End(mus())
	fmt.Printf("%v", m)
}
