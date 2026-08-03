# go-load-balancer

A Layer 7 (HTTP) reverse-proxy load balancer written in Go — with a lock-free
request hot path, pluggable balancing algorithms, and active + passive health
checking.

**What it is:** a small, focused, well-tested load balancer that demonstrates
the systems concerns of the problem — concurrency, health management, and
observability — in a form you can read, run, and benchmark.

**What it is not:** a service mesh, an API gateway, or an nginx/Envoy
replacement. TLS termination, HTTP/2, gRPC, rate limiting, and config hot-reload
are intentionally out of scope (see [Roadmap](#roadmap)).

## Features

- **Lock-free hot path** — the healthy-backend set is published via an
  `atomic.Pointer` snapshot; each request reads it with a single atomic load, no
  mutex. Per-backend counters (in-flight, health, failures) are `sync/atomic`.
- **Pluggable algorithms** — round-robin, least-connections, consistent-hashing
  (virtual-node ring, client-IP affinity), and **P2C-EWMA** (power-of-two-choices
  scored by a latency EWMA — the technique used by Finagle and Envoy).
- **Active + passive health checking** — periodic probes plus transport-failure
  feedback from real traffic; automatic eject and recover with hysteresis
  thresholds.
- **Idempotent retries** — a failed transport attempt retries on another backend
  only when no bytes have reached the client and the method is idempotent.
- **Observability** — a JSON `/metrics` endpoint and one structured `slog` log
  line per request.
- **Graceful shutdown** — drains in-flight requests within a configurable
  timeout.

## Quick start

Build:

```bash
go build -o bin/lb ./cmd/lb
```

Run the demo (three terminals):

```bash
# terminal 1 & 2 — two dummy backends
go run ./examples/basic/backend -name A -addr :9001
go run ./examples/basic/backend -name B -addr :9002

# terminal 3 — the load balancer
go run ./cmd/lb -config examples/basic/config.yaml
```

Send traffic:

```bash
curl localhost:8080    # handled by A
curl localhost:8080    # handled by B
curl localhost:8080/metrics
```

## Configuration

```yaml
listen: ":8080"                 # address the balancer listens on
algorithm: round_robin          # round_robin | least_connections | consistent_hash | p2c_ewma
shutdown_timeout: 5s            # drain window on shutdown
backends:
  - url: http://localhost:9001
    weight: 1                   # reserved for future weighted strategies
  - url: http://localhost:9002
health:
  path: /healthz                # active probe path
  interval: 2s                  # probe interval
  timeout: 1s                   # probe timeout
  healthy_threshold: 2          # consecutive successes to recover
  unhealthy_threshold: 3        # consecutive failures to eject
  passive_threshold: 5          # transport failures from live traffic before eject
proxy:
  max_retries: 1                # extra attempts on other backends (idempotent only)
transport:
  max_idle_conns: 100           # global idle connection pool
  max_idle_conns_per_host: 100  # raise from the default of 2 to avoid churn
  idle_conn_timeout: 90s
```

### Balancing algorithms

- `round_robin` — even rotation via one atomic counter.
- `least_connections` — the backend with the fewest in-flight requests.
- `consistent_hash` — client-IP affinity on a virtual-node hash ring.
- `p2c_ewma` — **power of two choices**: sample two random backends and pick the
  lower-cost one, where `cost = (latency_EWMA + 1) * (in_flight + 1)`. This
  reacts to latency (not just connection count) and avoids the herd behaviour of
  always choosing the global minimum. Latency is tracked per backend as an
  exponentially-weighted moving average updated on every response.

## Architecture

```
                 ┌──────────────────────────────────────────────┐
                 │                 Load Balancer                 │
   client ──────▶│  http.Server ──▶ Algorithm.Pick ──▶ Proxy ────┼──▶ backend[i]
                 │        │              ▲                        │
                 │        ▼              │ atomic snapshot        │
                 │    /metrics       BackendPool ◀── HealthChecker (goroutines)
                 └──────────────────────────────────────────────┘
```

### Concurrency model (the key design decision)

The set of *healthy* backends is read on **every request** and mutated **rarely**
(a health flip or config reload). This is a read-heavy / write-rare pattern, so:

- the pool holds an **immutable snapshot** (`[]*Backend`) behind an
  `atomic.Pointer`; readers do one atomic load — **no lock, no contention**,
  scaling with cores;
- writers build a **new** slice and atomically swap the pointer
  (copy-on-write); writes are rare, so the copy cost is irrelevant;
- per-backend mutable state (in-flight count, health, passive failures) uses
  `sync/atomic`, so incrementing an in-flight counter never takes a lock.

The alternative — a single `RWMutex` around the backend list — would put lock
acquisition on every request. The atomic-snapshot approach keeps the hot path
lock-free; a `-race` stress test (`internal/server/concurrency_test.go`) hammers
it under concurrent health flips.

The reasoning behind this model, the consistency window it accepts, and the
P2C-EWMA scoring choice are written up in [docs/design-notes.md](docs/design-notes.md).

## Benchmarks

Machine: Apple Silicon (darwin/arm64), Go 1.26.1. Reproduce with the commands
shown; numbers vary by machine.

**Selection microbenchmarks** — `go test -bench=. -benchmem ./internal/balancer/`

| Algorithm          | ns/op | allocs/op |
|--------------------|------:|----------:|
| Round-robin        |  3.4  |     0     |
| Least-connections  |  5.2  |     0     |
| Consistent-hashing | 14.5  |     0     |

All selection paths are allocation-free.

**End-to-end throughput** — `ab -n 20000 -c 50` against a no-op backend on
localhost, 0 failed requests in every run:

| Target                                   | req/s   | p99 (ms) |
|------------------------------------------|--------:|---------:|
| Direct backend (baseline)                | 25,900  |    3     |
| Through LB — default transport           |  9,100  |   16     |
| Through LB — tuned transport             | 16,900  |    6     |

The default `http.Transport` caps idle connections per host at **2**, which
forces connection churn under concurrency. Sharing one tuned transport
(`MaxIdleConnsPerHost = 100`) nearly **doubled throughput** (9.1k → 16.9k req/s)
and cut p99 latency from 16 ms to 6 ms — dropping proxy overhead from ~3× to
~1.5×. On localhost the backend does no real work, so this is a worst-case view
of the remaining proxy hop; with backends doing real work over a network the
relative overhead is smaller still.

## Known simplifications (V1)

- The consistent-hash ring rebuilds when the healthy set **size** changes; a
  production ring would key rebuilds on set identity.
- A backend returning HTTP 5xx is treated as a response (passed through), not a
  passive failure.
- The latency EWMA is a simple fixed-alpha average; a time-decayed peak-EWMA
  would react faster to sudden latency spikes.
- Weights are parsed but not yet used by a weighted strategy.

## Roadmap

- TLS termination, HTTP/2, and gRPC proxying
- Weighted and weighted-least-connections strategies
- Time-decayed peak-EWMA scoring for P2C
- Rate limiting and config hot-reload
- Prometheus metrics format and richer latency histograms

## Project layout

```
cmd/lb/              entrypoint
internal/backend/    Backend + lock-free Pool
internal/balancer/   Algorithm interface + strategies
internal/proxy/      selection, reverse proxy, retry, logging
internal/health/     state machine + active/passive checker
internal/metrics/    counters + /metrics
internal/config/     YAML config + validation
internal/server/     wiring + graceful shutdown
examples/basic/      runnable demo
```

## License

MIT
