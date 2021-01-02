Discrete Concurrency Library
===========================

Formal queueing theory has some interesting insights, but trying to extract a model out of a real system is problematic.
Measuring service times directly is hard.  Simply simiulating a system and instrumenting the simulation, or
measuring a real system are useful approaches to extracting insights.

If instead you can simply supply task measurements, in a similar fashion to Spans (Prometheus counter and gauge),
it is possible to get visualizations and measurements from the data.  One of the more interesting
measurements is the Universal Scaling Law.

- Get a scatter plot of load versus throughput.
- With the given points in the scatter plot, average throughput as a function of load should be enough to curve fit to the USL
- A curve fit to the USL will allow a forecast of where the service stops increasing throughput when it is given more load to handle.
- Comparing the USL to linear scaling gives a direct efficiency measurement.  Given that the costs go up linearly with the number of servers handling the load, it can be calculated not only what is the maximum throughput that is possible, but at what point it is too expensive to buy minor gains in throughput.
- The USL curve implicitly measures contention, which ends up being waiting in queues.  We can use this information to calculate latencies.
- The USL curve implicitly measures coherence (cross-talk), which is caused by having to come to agreement with a percentage of all existing servers.  Coherence not only limits throughput, but it can cause adding more resources to actually decrease throughput. 

Preliminaries:

- A span is a measurement put in when a task reports progress:  `(start, stop, count, load)`, where `load==1` for observing counts going up or `load==0, count==0` for simply ensuring that all time is accounted for when measurements are being taken.
- `start` is int64.
- `stop` is int64.
- `count` could be int64 or float64.  I currently use int64.  A `count` should be greater than or equal to zero.
- `start < stop`, so that there are no zero-length intervals.  It is up to you to choose units such that happens conveniently.

When a system is under observation:

- The fraction of _observed_ time in which it has load greater than zero, is Utilization.  For jobs serviced against a CPU, this would be CPU utilization, a number from 0 to 1.
- We need to keep _unobserved_ time out of metrics.  That means that we need to note when observation actually begins and ends.
- The correct _weighted_ average is the total `count` divided by the total duration `(stop - start)`.  When you have concurrency, it's even more subtle than this.  There is the total duration of time for tasks that may overlap in time (the throughput you would have if there were no concurrency at all - what the datacenter is concerned with, because it's related to power consumption), versus total clock time (for the concurrently sped-up throughput - what the user is concerned with, because it's related to the performance observed by a user, regardless of what other users are seeing).  Keep a running total of both separately.  You cannot take averages of averages, as this will not weight them correctly.
- The `count/(stop - start)` is the rate at which the counter increases.

Note that:

- If we only report the rate, without the duration, we have no idea by how much the count increased over the duration
- If we only report the rate and duration without a `start` time, then we cannot properly calculate _overlaps_ in the data.  Overlaps are the heart of concurrency and queueing.

The goal is an object that has an accurate API like:

- `metrics.StartObservation(start)` to begin including all time, including idle time, in calculations.
- `metrics.AddObservation(count, stop, start)` add in observations.  This should happen while observation is actually started to properly include idle time.
- `metrics.StopObservation(stop)`  stop collecting data.
- 'metrics.GetThroughputVsLoad()` function to calculate throughput at observed loads.

Since a count is reported after it stops, we must be able to report data out of order.  
The whole problem with concurrency is that concurrent tasks happen in arbitrary order relative to each other, subject to scheduling.
Typically, the units for `start` and `stop` would be such that `start` and `stop` for a `count` can never be the same.  UnixNano should be fine for this.

- The units of counter rates `count/(stop - start)` is in `count/time`.  To properly average these, the `count` and `duration` must be _independently_ summed, and the weighted average taken as `totalCount/totalDuration`.
- With properly weighted averages, we have a way to start calculating statistical items of interest, such as standard deviation.

Insights
============

An algorithm to handle this correctly would:

- Periodically turn each reported span `(start, stop, count, load)` into a statistical report.
- We cannot just sort the spans by `start`, because there are overlaps due to `stop`; and it is these overlaps that are the whole point of the measurements.
- So, simply process the spans by inserting into a NEW data structure that inserts the span TWICE, once for `start` and once for `stop`, at clock time `at`.
- Sort the whole thing by these `at`.  The `load` is accumulated for each `at` as a running total, with `start==at` being to add it, and `stop==at` being to subtract it.
- Including `count` is more complex.  We are really dealing with a rate rather than the actual count.  We want to increment throughput (ie: rate) by `counter/(stop-start)` when `start==at`, and decrement when `stop==at`.  Within floating poing errors, the throughput and `load` should be back to exactly zero after iterating over all reported spans.
- Note that `at` will be duplicated.  A single `at` entry that accumulates rate and `load` should be made to remove duplicated `at`.

Given this, we have a sequence of `(at, throughput, load)`, where the graph `(load, throughput)` is what we need to fit to the USL.

This should completely characterize the measured scalability.  It tells us how much concurrency (load) was actually handled, with the ratio of `load/throughput` at any time being how efficiently resources are being used to handle the load. 

