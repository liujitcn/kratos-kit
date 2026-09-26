# Cache

This module provides memory and Redis implementations of the shared cache interface.

## Rate limits

`Cache.TakeTokenBuckets` atomically checks and consumes every requested token bucket. The operation is all-or-nothing: if any bucket rejects the request, no bucket is consumed. It returns the decision, the longest retry delay, and an error. Each request specifies a unique key, a refill rate in tokens per second, and a burst capacity.

The memory implementation serializes bucket updates within the process. Redis and Redis Cluster use a Lua script so the check and update run atomically on the Redis server.

## Shared cache revisions

`ReadRevision` reads a numeric revision and treats a missing key as revision `0`. It reports whether the revision can be used; invalid values and cache errors disable the cached snapshot. `IncrementRevision` atomically advances the revision so other nodes can move to a new cache key.
