# Design notes: keeping the request path lock-free

A load balancer is boring until you look at what every request touches. This one
forwards HTTP to a pool of backends, and on the way through it reads exactly one
shared thing: the set of backends currently considered healthy. That set is read
on every request and written almost never — only when a health check flips a
backend or the config reloads. Getting the concurrency right for that asymmetry
is most of what separates this project from a tutorial, so it's worth writing
down what I did and, more importantly, what I chose not to do.

## The obvious version is the wrong one

The first instinct is a slice of backends behind a `sync.RWMutex`: readers take
the read lock, the health checker takes the write lock. It's correct, and it's
what most examples show.

It also puts a lock on the hot path. An `RWMutex` read lock is not free under
concurrency — readers still coordinate on shared state, and that cache line
bounces between cores. When the thing you're protecting is read tens of thousands
of times a second and written a few times a minute, you're paying a
synchronization cost on every request to defend against a write that almost never
happens. That's backwards.

## Publish an immutable snapshot instead

The set of healthy backends is a value, not a place, so I treat it as one. The
pool holds an immutable `[]*Backend` behind an `atomic.Pointer`, and readers just
load it:

```go
func (p *Pool) Healthy() []*Backend { return *p.healthy.Load() }
```

One atomic load. No lock, no contention, and it scales with cores because nothing
is shared-mutable on the read side. When health changes, the writer builds a
*new* slice and swaps the pointer:

```go
func (p *Pool) rebuildHealthy() {
    all := *p.all.Load()
    h := make([]*Backend, 0, len(all))
    for _, b := range all {
        if b.IsHealthy() {
            h = append(h, b)
        }
    }
    p.healthy.Store(&h)
}
```

This is copy-on-write, and the usual objection is that copying is expensive. It
isn't, because the copy happens on the rare path. A write costs O(n) to rebuild
the slice; a read costs a single atomic load. Reads outnumber writes by four or
five orders of magnitude, so spending more on the write to make the read cheap is
exactly the trade you want. A small mutex serializes writers so two health flips
don't race to rebuild — but no request ever touches that mutex.

Per-backend mutable state — in-flight count, the health flag, the latency EWMA —
lives on each `*Backend` as `sync/atomic` fields for the same reason.
Incrementing the in-flight counter on a request must not take a lock, so it
doesn't:

```go
func (b *Backend) IncInFlight() { b.inFlight.Add(1) }
```

## The consistency you get, and the consistency you don't

Snapshots buy speed by giving up a guarantee, and it's worth being honest about
which one. A request can load the healthy snapshot and, in the microsecond before
it forwards, the health checker can mark that backend unhealthy. The request goes
to it anyway. No locking closes that window without putting a lock back on the hot
path, and closing it isn't worth it, because the system already tolerates a bad
pick: a transport failure feeds the passive-failure path and, for idempotent
requests, retries on another backend. Health state is eventually consistent, and
the retry path is what makes "eventually" good enough. Reaching for stronger
consistency here would be solving a problem the design already absorbs.

## Choosing a backend: why not just "least loaded"

Round-robin ignores load. The next obvious step, least-connections, picks the
backend with the fewest in-flight requests, and it has a failure mode: every
chooser sees the same global minimum at the same instant and piles onto it, so
the "least loaded" backend becomes a hotspot until the counters catch up. That's
the thundering herd of greedy balancing.

Power of two choices sidesteps it. Instead of scanning for the global minimum,
sample two backends at random and take the better of the two. Different requests
sample different pairs, so there's no coordinated stampede, and the cost is O(1)
instead of O(n). The result (Mitzenmacher) is that two random probes get you most
of the benefit of perfect knowledge for almost none of the cost.

"Better" is the interesting part. I score each candidate by latency, not just
connection count:

```
cost(b) = (LatencyEWMA(b) + 1) * (InFlight(b) + 1)
```

The EWMA is an exponentially-weighted moving average of recent request latency,
updated on every response. A backend that is technically accepting connections
but answering slowly gets a rising cost and sheds traffic before it starts
failing outright — something connection count alone can't see. The `+1` terms
keep a fresh backend (EWMA of zero) from looking infinitely attractive; with no
samples yet the score collapses to in-flight count, so it behaves like
least-connections until latency data exists. This is close to what Finagle and
Envoy do.

I kept the EWMA simple: a fixed smoothing factor rather than a time-decayed peak.
A peak-EWMA reacts faster to a sudden spike and is the honest next step. I left it
out because the simple version is enough to show the idea and easier to reason
about, and I'd rather ship something I fully understand than something that looks
more sophisticated in the README.

## One number worth the whole exercise

The least glamorous finding was the most useful. Go's default `http.Transport`
caps idle connections per host at two. Two. Under fifty concurrent clients that
means near-constant connection setup and teardown to the backends, and it showed
up as a flat ~3x throughput penalty against hitting a backend directly. Sharing
one transport with `MaxIdleConnsPerHost` raised to 100 moved throughput from ~9k
to ~17k requests per second and cut p99 latency from 16 ms to 6 ms on the same
machine. No algorithmic cleverness — just a default that was wrong for this
workload. Most real performance work looks like this, and I'd rather show the
boring true number than a flattering synthetic one.
