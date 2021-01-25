package main

import(
  "math"
  "fmt"
  "time"
  "strings"
  "math/rand"
)

type Instant struct {
	At int64
	ThroughputDelta float64
	LoadDelta int64
}

func (i *Instant) String() string {
  return fmt.Sprintf("%d:, dX:%f, dn:%d", i.At, i.ThroughputDelta, i.LoadDelta)
}

type Reporter struct {
  // sorted by Data[i].At
  Data []Instant
  InsertAt int
  MinAt int64
  MaxAt int64
  ThroughputInit float64
  LoadInit int64
  Alpha float64
  Beta float64
  Gamma float64
}

func (r *Reporter) X(da,db,dg float64, ni int64) float64 {
  a := r.Alpha + da
  b := r.Beta + db
  g := r.Gamma + dg
  n := float64(ni)
  return (n * g) / (1 + a*(n-1) + b*n*(n-1))
}

func (r *Reporter) InBoundsX(da, db, dg float64) bool {
  a := r.Alpha + da
  b := r.Beta + db
  g := r.Gamma + dg
  return a>=0 && a<=1 && g>=0 && b>=0 
}

func (r *Reporter) Err2f(da, db, dg float64) float64 {
  err := float64(0)
  W := float64(0)
  X := float64(0)
  n := int64(0)
  for t := 0 ; t < len(r.Data)-1; t++ {
    X += r.Data[t].ThroughputDelta
    n += r.Data[t].LoadDelta
    if n > 0 {
      e := X - r.X(da,db,dg,n)
      w := float64(r.Data[t+1].At - r.Data[t].At)
      err += e*e*w
      W += w
    }
  }
  return err/W
}

func (r *Reporter) Fit() float64 {
  // Fit the parameters
  err := r.Err2f(0,0,0)
  step := 0.001
  iterations := 100000
  for i := 0; i < iterations; i++ {
        // try something random, and use it if it's an improvement
        da := float64(rand.Int()%3 - 1) * step * err
        db := float64(rand.Int()%3 - 1) * step * err
        dg := float64(rand.Int()%3 - 1) * step * err
        //fmt.Printf("da:%f, db:%f, dg: %f\n", da, db, dg)
  	err2 := r.Err2f(da,db,dg)
        ib := r.InBoundsX(da,db,dg)
        //fmt.Printf("err: %f, err2: %f, step: %f\n, inbounds: %t\n",err, err2, step, ib)
        if err2 < err && ib {
          r.Alpha += da
          r.Beta += db
          r.Gamma += dg
          err = err2
        }
  }
  return err
}

func (r *Reporter) String() string {
  result := make([]string,0)
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
    at := r.Data[i].At
    X += r.Data[i].ThroughputDelta
    n += r.Data[i].LoadDelta
    if X > maxThroughput {
      maxThroughput= X
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
    result = append(result, fmt.Sprintf("%d: X:%f, n:%d",at,X,n))
  }
  avgThroughput := totalWork/float64(totalTime)
  result = append(
    result, 
    fmt.Sprintf(
      "X in [%f .. %f], average: %f", 
      minThroughput, 
      maxThroughput, 
      avgThroughput,
    ),
  )
  avgLoad := totalLoad/float64(totalTime)
  result = append(
    result, 
    fmt.Sprintf(
      "n in [%d .. %d], average: %f",
      minLoad,
      maxLoad,avgLoad,
    ),
  )

  errInit := r.Err2f(0,0,0)
  err := r.Fit()
  result = append(
    result, 
    fmt.Sprintf(
      "alpha: %f, beta: %f, gamma: %f, err: %f, errInit: %f", 
      r.Alpha, 
      r.Beta, 
      r.Gamma,
      err,
      errInit,
    ),
  )
  nPeakF := math.Sqrt((1-r.Alpha)/r.Beta)
  nPeak := int64(nPeakF)
  Xpeak := r.X(0,0,0,nPeak)
  XpeakEfficiency := Xpeak/(float64(nPeak)*r.Gamma)
  result = append(
    result,
    fmt.Sprintf(
      "peakLoad: %f, peakThroughput: %f, peakThroughputEfficiency: %f",
      nPeakF,
      Xpeak,
      XpeakEfficiency, 
    ),
  )
  return strings.Join(result,"\n")
}

func NewReporter() *Reporter {
	r := &Reporter{}
	return r
}

func (r *Reporter) Add(at int64, throughputdelta float64, loaddelta int64) {
  if at < r.MinAt {
    r.MinAt = at
  }
  if at > r.MaxAt {
    r.MaxAt = at
  }
  N := len(r.Data)-1
  n := N
  if n == -1 {
    // Inserting the very first item
    r.Data = append(r.Data, Instant{
      At: at,
      ThroughputDelta: throughputdelta,
      LoadDelta: loaddelta,
    })
    return 
  }
  // Find our insert point, or increment an exact time match
  for n >= 0 {
    if r.Data[n].At == at  {
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
    At: at,
    ThroughputDelta: throughputdelta,
    LoadDelta: loaddelta,
  })
  for i := N; i > n; i-- {
    t := r.Data[i+1]
    r.Data[i+1] = r.Data[i]
    r.Data[i] = t
  } 
}

func (r *Reporter) Do(start int64, stop int64, throughput float64, load int64) {
  r.Add(start, throughput, load)
  r.Add(stop, -throughput, -load)
}

func (r *Reporter) Begin(at int64) {
  r.Add(at, 0, 0)
}

func (r *Reporter) End(at int64) {
  r.Add(at, 0, 0)
}

func Tstamp() int64 {
	return time.Now().UnixNano()
}

func main() {
  r := NewReporter()
  r.Begin(0)        //start observing at time 0
  r.Do(2,12, 1.5,1)  //at 2..6, 1.5 throughput from 1 load
  r.Do(3,13, 1.5,1)  //at 2..6, 1.5 throughput from 1 load
  r.Do(4,14, 1.5,1)  //at 2..6, 1.5 throughput from 1 load
  r.Do(5,15, 1.5,1)  //at 2..6, 1.5 throughput from 1 load
  r.Do(6,16, 1.5,1)  //at 2..6, 1.5 throughput from 1 load
  r.Do(7,17, 1.5,1)  //at 2..6, 1.5 throughput from 1 load
  r.Do(8,19, 1.5,1)  //at 2..6, 1.5 throughput from 1 load
  r.Do(9,20, 1.5,1)  //at 2..6, 1.5 throughput from 1 load
  r.Do(10,21, 1.5,1)  //at 2..6, 1.5 throughput from 1 load
  r.Do(11,21, 1.5,1)  //at 2..6, 1.5 throughput from 1 load
  r.Do(12,22, 1.5,1)  //at 2..6, 1.5 throughput from 1 load
  r.End(30)         //stop ovserving at time 10
/*
  r.Add(0, 0, 0)
  r.Add(1, 1, 1)
  r.Add(2, 1, 1)
  r.Add(3, 1, 1)
  r.Add(4, 1, 1)
  r.Add(5, 1, 1)
  r.Add(6, 1, 1)
  r.Add(7, 1, 1)
  r.Add(8, 1, 1)
  r.Add(9, 1, 1)
  r.Add(10,0, 0)
*/
  fmt.Printf("%v",r)
}
