package main

import (
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"strings"
	"sync"
	"time"
)

type Instant struct {
	At              int64
	ThroughputDelta float64
	LoadDelta       int64
}

type Reporter struct {
	// sorted by Data[i].At
	Data           []Instant
	InsertAt       int
	MinAt          int64
	MaxAt          int64
	ThroughputInit float64
	LoadInit       int64
	F              Gd
}

func (r *Reporter) String() string {
	result := make([]string, 0)
	maxLoad := int64(0)
	minLoad := int64(0)
	maxThroughput := float64(0)
	minThroughput := float64(0)
	X := r.ThroughputInit
	totalLoad := float64(0)
	totalWork := float64(0)
	totalTime := int64(0)
	n := r.LoadInit
	// Accumulate data points
	for i := 0; i < len(r.Data); i++ {
		//at := r.Data[i].At
		X += r.Data[i].ThroughputDelta
		n += r.Data[i].LoadDelta
		if X > maxThroughput {
			maxThroughput = X
		}
		if X < minThroughput {
			minThroughput = X
		}
		if n > maxLoad {
			maxLoad = n
		}
		if n < minLoad {
			minLoad = n
		}
		if i+1 < len(r.Data) {
			interval := r.Data[i+1].At - r.Data[i].At
			totalTime += interval
			totalLoad += float64(n) * float64(interval)
			totalWork += X * float64(interval)
		}
	}
	avgThroughput := totalWork / float64(totalTime)
	result = append(
		result,
		fmt.Sprintf(
			"X in [%f .. %f], average: %f",
			minThroughput,
			maxThroughput,
			avgThroughput,
		),
	)
	avgLoad := totalLoad / float64(totalTime)
	result = append(
		result,
		fmt.Sprintf(
			"n in [%d .. %d], average: %f",
			minLoad,
			maxLoad, avgLoad,
		),
	)

	// dump a (load,throughput) graph
	throughputByLoad := make([]float64, maxLoad+1)
	throughputWeightByLoad := make([]float64, maxLoad+1)
	X = r.ThroughputInit
	n = r.LoadInit
	for i := 0; i < len(r.Data)-1; i++ {
		X += r.Data[i].ThroughputDelta
		n += r.Data[i].LoadDelta
		w := float64(r.Data[i+1].At - r.Data[i].At)
		throughputByLoad[n] += X * w
		throughputWeightByLoad[n] += w
	}
	for n := 0; n < len(throughputByLoad); n++ {
		throughputByLoad[n] = throughputByLoad[n] / throughputWeightByLoad[n]
	}
	errInit := r.F.errf(0, 0, 0, throughputByLoad)
	err := r.Fit(throughputByLoad)
	result = append(
		result,
		fmt.Sprintf("gamma: %f", r.F.Gamma),
	)
	result = append(
		result,
		fmt.Sprintf("alpha: %f", r.F.Alpha),
	)
	result = append(
		result,
		fmt.Sprintf("beta: %f", r.F.Beta),
	)
	result = append(
		result,
		fmt.Sprintf("err: %f, errInit: %f", err, errInit),
	)
	nPeakF := math.Sqrt((1 - r.F.Alpha) / r.F.Beta)
	nPeak := int64(nPeakF)
	Xpeak := r.F.X(0, 0, 0, nPeak)
	XpeakEfficiency := Xpeak / (float64(nPeak) * r.F.Gamma)
	result = append(
		result,
		fmt.Sprintf(
			"peakLoad: %f, peakThroughput: %f, peakThroughputEfficiency: %f",
			nPeakF,
			Xpeak,
			XpeakEfficiency,
		),
	)

	for n := 0; n < len(throughputByLoad); n++ {
		result = append(
			result,
			fmt.Sprintf(
				"%d, %f",
				n,
				throughputByLoad[n],
			),
		)
	}

	result = append(result, "")
	return strings.Join(result, "\n")
}

func NewReporter() *Reporter {
	r := &Reporter{}
	return r
}

func (r *Reporter) At(at int64, throughputdelta float64, loaddelta int64) {
	if at < r.MinAt {
		r.MinAt = at
	}
	if at > r.MaxAt {
		r.MaxAt = at
	}
	N := len(r.Data) - 1
	n := N
	if n == -1 {
		// Inserting the very first item
		r.Data = append(r.Data, Instant{
			At:              at,
			ThroughputDelta: throughputdelta,
			LoadDelta:       loaddelta,
		})
		return
	}
	// Find our insert point, or increment an exact time match
	for n >= 0 {
		if r.Data[n].At == at {
			// Inserting into matching bucket
			r.Data[n].ThroughputDelta += throughputdelta
			r.Data[n].LoadDelta += loaddelta
			return
		}
		if r.Data[n].At < at {
			break
		}
		n--
	}
	// We are sitting to the right of item less than us, or at index -1
	// Append it to the end, and sink it down into its place
	r.Data = append(r.Data, Instant{
		At:              at,
		ThroughputDelta: throughputdelta,
		LoadDelta:       loaddelta,
	})
	for i := N; i > n; i-- {
		t := r.Data[i+1]
		r.Data[i+1] = r.Data[i]
		r.Data[i] = t
	}
}

func (r *Reporter) Do(start int64, stop int64, throughput float64, load int64) {
	r.At(start, throughput, load)
	r.At(stop, -throughput, -load)
}

func (r *Reporter) Begin(at int64) {
	r.At(at, 0, 0)
}

func (r *Reporter) End(at int64) {
	r.At(at, 0, 0)
}

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
