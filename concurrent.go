package main

import (
 "log"
 "fmt"
 "sort"
 "time"
 "sync"
 "math/rand"
 "runtime"
)

/**
  This supports a basic form of discrete queueing theory.
  Given average throughput at a given load, we can calculate the Universal Scaling Law from it.
  This will let us forecast where scaling up will stop, as efficiency losses cause scaling up to slow the system down.

  Spans with a count get submitted.  This is the raw user data.  It can arrive in any order.
   start stop count load
    0 5 22 1    // from time 0 to time 5, we uploaded 22 bytes consuming 1 thread
    5 9 23 1
    6 10 18 1

  the stop is NOT included in the range, and data MUST have stop > start
  to prevent a zero interval.  Implicitly, the range happens twice in the data.
  It happens on start with a positive counter, and happens again on stop with a negative counter.

  Note that if a later event start subtracts an earlier start, we get an interval, just like the stop minus start interval.
  The (count / (stop - start)) is the rate that count goes up.

  So, when the events are double-counted by stop and start, 
  we can accumulate the count rate and load as we go.
  The last time that matches is the total for that time.

  Calculating cumulative load, and cumulative throughput....
  a=0
     0 +  0  5 22 1   1  22/(5-0)=a    = 22/5
     5 -  0  5 22 1   1  b+22/(5-0)=c  = 22/5 - 22/5 = 0
     5 +  5  9 23 1   1  c+23/(9-5)=d  = 23/4
     6 +  6 10 18 1   2  d+18/(10-6)=e = 23/4 + 18/4 = 5/4
     9 -  5  9 23 1   1  e+23/(9-5)=f  = 23/4 + 18/4 - 23/4 = 18/4 
    10 -  6 10 18 1   0  f+18/(10-6)=g = 18/4 - 18/4 - 0

 Notice that the load temporarily spiked to 2 due to an overlap in intervals, and that we started and ended count at 0.
 Because the load of each span we observed had a load of at least 1, the utilization is 100%, which is the fraction of time that load is not zero.
 If load for a span is 0, then its count must also be zero so that count/load is defined.

 */
type Dt int64
type Load int64
type Count int64

type Span struct {
	Start       Dt
	Stop        Dt
	Count       Count
	Load        Load
}

type Metrics struct {
	Data      []Span
	observeStart Dt
}

func NewMetrics() *Metrics {
	m := &Metrics{}
	return m
}

func (m *Metrics) log(mask string, args ...interface{}) {
	log.Printf(mask, args...)
}

func (m *Metrics) StartObserving(start Dt) {
	m.observeStart = start
}

func (m *Metrics) StopObserving(stop Dt) {
	m.Data = append(m.Data, Span{Start: m.observeStart, Stop: stop, Load: 0})
}

func (m *Metrics) Add(start Dt, stop Dt, count Count) {
	if start >= stop {
		// hmm... just for safety, reject calls where stop==start.  
		// perhaps panic is too much for this
		return
	}
	m.Data = append(m.Data, Span{Start: start, Stop: stop, Count: count, Load: 1})
}

// A span will be represented by two waypoints ... one for start, and one for stop

type Section struct {
	Start      Dt
	Duration   Dt
	Load       Load
	CountRate float64
}

type PerformanceMetrics struct {
	Data []Section
}

func (m *Metrics) Calculate() *PerformanceMetrics {
	type waypoint struct {
		At   Dt
		Span Span
	}
	pm := PerformanceMetrics{}
	//Make a set of waypoints to iterate the data
	wp := make([]waypoint, 0)
	for i := range m.Data {
		// We will be modifying these to distribute counter accross buckets
		p := m.Data[i]
		wp = append(wp, waypoint{p.Start, p})
		wp = append(wp, waypoint{p.Stop, p})
	}
	waypointSort := func(i, j int) bool {
		ai := wp[i].At
		aj := wp[j].At
		di := wp[i].Span.Stop
		return (ai < aj) || ((ai == aj) && (di == ai))
	}
	sort.Slice(wp, waypointSort)
	// compact out redundancies and summarize
	countRate := float64(0)
	load := Load(0)
	duration := Dt(0)
	sections := make([]Section, 0)
	for i, s := range wp {
		if i+1 < len(wp) {
			duration = wp[i+1].At - wp[i].At
		} else {
			duration = 0
		}
		loadChange := s.Span.Load
		countChange := s.Span.Count
		durationChange := (s.Span.Stop - s.Span.Start)
		countRateChange := float64(countChange)/float64(durationChange)
		if s.At == s.Span.Start {
			countRate += countRateChange
			load += loadChange
		} else {
			countRate -= countRateChange
			load -= loadChange
		}
		if i+1 < len(wp) && wp[i].At == wp[i+1].At {
			// skip it to take last value
		} else {
			sections = append(sections, Section{
				Start:      s.At,
				Load:       load,
				CountRate: countRate,
				Duration:   duration,
			})
		}
	}
	pm.Data = sections
	return &pm
}

type ThroughputAtLoad struct {
	Load Load
	TotalWeightedThroughput float64
	TotalThroughputWeight float64
}

type Answer []ThroughputAtLoad

func (m *PerformanceMetrics) ThroughputAtLoad() Answer {
	copied := make([]Section,len(m.Data))
	_ = copy(copied,m.Data)

	// sort the time segments to squash into average throughput
	loadSort := func(i, j int) bool {
		return copied[i].Load < copied[j].Load
	}
	sort.Slice(copied, loadSort)
	// accumulate answers that average by load
	previousLoad := Load(-1)
	answer := make(Answer,0)
	var a ThroughputAtLoad
	for _, s := range copied {
		if s.Load != previousLoad {
			// flush previuos result
			if previousLoad != -1 {
				answer = append(answer, a)
			}
			// prepare to do weighted average
			a.Load = s.Load
			a.TotalWeightedThroughput = 0
			a.TotalThroughputWeight = 0
		}
		// accumulate averages...  (count/duration)*duration weighted count
		a.TotalWeightedThroughput += float64(s.CountRate) * float64(s.Duration)
		a.TotalThroughputWeight += float64(s.Duration)
		previousLoad = s.Load
	}
	return answer
}

func (answer Answer) Write() {
	fmt.Printf("load, throughput\n")
	// report in milliseconds, though we clocked it in nanoseconds to prevent overlaps
	normalizeAt1 := float64(1000000)
	for _, a := range answer {
		throughput := normalizeAt1 * (a.TotalWeightedThroughput / a.TotalThroughputWeight)
		fmt.Printf("%d, %f\n", a.Load, throughput)
	}
}

// We ASSUME that we can't return the same value twice, which prevents 0 length intervals
func ns() Dt {
	return Dt(time.Now().UnixNano())
}

// A simple test to exercise the library
func schedTest(m *Metrics) {
	sharedBottleneckQueue := 16
	concurrentRequests := 512
	ch := make(chan int, sharedBottleneckQueue)
	wg := sync.WaitGroup{}
	for i := 0; i < concurrentRequests; i++ {
		wg.Add(1)
		go func() {
			for j := 0; j < 4; j++ {
				runtime.Gosched()
				start := ns()
				tasks := Count(rand.Intn(1000))
				for l := 0; l < int(tasks); l++ {
					ch <- 5
					runtime.Gosched()
					_ = <-ch
				}
				finish := ns()
				m.Add(start, finish, tasks)
				runtime.Gosched()
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func main() {
	m := NewMetrics()
	m.StartObserving(ns())
	schedTest(m)
	m.StopObserving(ns())
	m.Calculate().ThroughputAtLoad().Write()
}

