# Compaction Design Review

Validation of [CANDIDATE_COMPACTION.md](CANDIDATE_COMPACTION.md) against the current codebase.

## 1. File layout

### What the doc proposes

```
logs/{partition-X}
candidates/
  pending/{partition-X}
  applied/{partition-X}
  dismissed/{partition-X}
  journal/{journal-partition}
```

### What exists

Logs live under `dk_logs/` with a two-level layout:

```
dk_logs/{YYYY}/{YYYY-MM-DD}_logs.jsonl
```

Partitions are **day-granularity**, built from the entry's `Timestamp` field (`jsonlstore.buildRelPath`).
The `logType` is hardcoded to `"logs"` when the `Store` creates the `JSONLWriter`.

### Issues

| # | Issue | Severity |
|---|-------|----------|
| 1.1 | **Partition format under-specified.** The doc says partitions should share "same time-start-time-end", but never names the actual scheme (`{YYYY}/{YYYY-MM-DD}_{logType}.jsonl`). Candidates must use the identical key scheme so the compactor can pair files by relative path. | Medium |
| 1.2 | **Root directory relationship unclear.** `dk_logs/` is the current log root. Where does `candidates/` live — beside `dk_logs/` under the data path, or nested inside it? Should be explicit. | Medium |
| 1.3 | **logType coupling.** The partition filename includes the logType segment (`_logs`). If candidates reuse the same `buildRelPath`, the type would still read `logs`. Candidate files need a distinct logType or a separate directory prefix to avoid ambiguity. | Low |

### Recommendation

Define the concrete layout relative to `dataPath`:

```
{dataPath}/
  dk_logs/{YYYY}/{YYYY-MM-DD}_logs.jsonl          # existing
  dk_candidates/
    pending/{YYYY}/{YYYY-MM-DD}_candidates.jsonl
    applied/{YYYY}/{YYYY-MM-DD}_candidates.jsonl
    dismissed/{YYYY}/{YYYY-MM-DD}_candidates.jsonl
    journal/{YYYY}/{YYYY-MM-DD}_journal.jsonl
```

---

## 2. Entry matching by ID

### What the doc says

> "The storage layer matches candidates to originals by ID."

### What exists

- `Entry.ID` is a UUID v4 assigned at append time (`command.AppendEntry`).
- The SQLite index is **file-level only** — it stores `(file, tags, min_ts, max_ts)`. There is no entry-level index.
- Retrieving an entry by ID requires scanning JSONL files.

### Issues

| # | Issue | Severity |
|---|-------|----------|
| 2.1 | **No efficient ID → file lookup.** To compact, you need to find which partition file holds the original entry. With the current index there's no way to do that without scanning all files. | High |
| 2.2 | **Candidate must carry locator metadata.** The doc says a candidate is "a full copy with edits applied, same ID". But it doesn't specify whether the candidate also stores the original's partition path or timestamp, which is needed to locate the original. | High |

### Recommendation

Two options (pick one):

1. **Candidate carries the original's `Timestamp` (and thus partition key).** Since partitions are day-bucketed by `Timestamp`, the compactor can derive the file path. This is the simplest approach — no index changes. It does require that the candidate's `Timestamp` field preserves the original value (or a separate `original_ts` field is added).

2. **Add an entry-level index table** (`entry_id TEXT PRIMARY KEY, file TEXT NOT NULL`). Updated on every append. Adds write-time overhead (~1 extra INSERT per entry) but enables fast ID lookups.

Option 1 is simpler and consistent with the "scan pending" approach the doc's open questions already lean toward.

---

## 3. Compaction process

### What the doc says

1. Replace matching entry in `logs` by ID → move candidate `pending → applied`.
2. Dismissed → move `pending → dismissed`.
3. Rewrite affected partitions atomically.
4. Append resolution to `journal`.

### What exists

- `JSONLWriter` only supports `Append` (O_APPEND) and `Read`. There is **no rewrite/replace API**.
- `JSONLWriter` caches open file descriptors in a `sync.Map`. Rewriting a file (write-temp + rename) would invalidate the cached FD.
- The `Store` facade has no per-partition coordination — only a process-level `flock`.

### Issues

| # | Issue | Severity |
|---|-------|----------|
| 3.1 | **Step ordering is unclear.** Step 1 says "replace … move candidate", step 3 says "rewrite atomically". The replacement IS the rewrite. Logical order should be: collect resolutions → rewrite partitions → move candidates → journal. | Medium |
| 3.2 | **FD cache invalidation.** After a partition is rewritten via temp-file + rename, the writer's cached `*os.File` is stale (points at the unlinked old inode). The compactor must coordinate with `JSONLWriter` to evict and reopen. | High |
| 3.3 | **Index maintenance after rewrite.** A compacted partition may have different tags or a shifted time range (e.g., the replaced entry had a unique tag). The index row must be recalculated. The doc doesn't mention this. | Medium |
| 3.4 | **Concurrent append during rewrite.** If an append lands on a partition that is being rewritten, it will succeed on the old FD (now unlinked) and the data will be lost when rename completes. Needs a per-partition lock or quiesce. | High |

### Locking analysis

Two problems to solve:

1. **Runtime coordination** — prevent appends and compaction from racing on the same partition.
2. **Crash recovery** — if the process dies mid-compaction, detect and recover on next startup.

The key insight: `flock()` with **separate file descriptors** within the same process provides proper shared/exclusive semantics across goroutines AND auto-releases on crash. Verified experimentally on Linux — `LOCK_EX` on fd2 blocks while fd1 holds `LOCK_SH` on the same file, even within the same process. This is the foundation of the recommended approach.

---

### Strategy E: Per-partition flock on sidecar `.lock` files (recommended)

Each partition gets a sidecar lock file:

```
dk_logs/2025/2025-03-29_logs.jsonl
dk_logs/2025/2025-03-29_logs.jsonl.lock    ← flock target
```

**Writer goroutines** open their own fd to the `.lock` file, acquire `LOCK_SH`, write, release `LOCK_SH`, close fd.
**Compactor** opens its own fd, acquires `LOCK_EX` (blocks until all `LOCK_SH` holders release), rewrites, releases `LOCK_EX`, closes fd.

```
Append:    fd = open(.lock) → flock(fd, LOCK_SH) → write entry → flock(fd, LOCK_UN) → close(fd)
Compact:   fd = open(.lock) → flock(fd, LOCK_EX) → rewrite     → flock(fd, LOCK_UN) → close(fd)
```

#### Why flock, not OFD/fcntl

| Property | `flock()` | `fcntl(F_SETLK)` POSIX | `fcntl(F_OFD_SETLK)` |
|----------|-----------|------------------------|----------------------|
| Intra-process conflict (separate fds) | **Yes** | No (process-wide) | Yes |
| Auto-release on crash | **Yes** (fd closed by OS) | Yes (process dies) | Yes (last fd closed) |
| Closing unrelated fd releases lock | **No** | Yes (dangerous!) | No |
| Kernel requirement | All Linux | All Linux | Linux ≥ 3.15 |
| Already in codebase (`pkg/flock`) | **Yes** | No | No |

`flock()` wins: correct intra-process semantics, crash-safe, no exotic kernel requirements, consistent with existing `pkg/flock` usage.

#### Why sidecar `.lock` files, not locking the JSONL file directly

Locks attach to the **inode**, not the path. After `rename(tmp, original)`, the original path points to a new inode — any lock held on the old inode is meaningless. A stable sidecar file that is never renamed avoids this problem. The `.lock` file's inode never changes.

#### Append-path overhead

Each append: `open()` + `flock(LOCK_SH)` + `flock(LOCK_UN)` + `close()` ≈ **3–5 µs** on Linux (two syscalls for flock, two for open/close).

For a personal logging service doing single-digit writes per second, this is negligible. If it ever matters, the sidecar fd can be cached per-goroutine or pooled — but premature optimization.

#### FD lifecycle during compaction

1. Compactor: `open(.lock)` → `flock(LOCK_EX)` — blocks until all writer `LOCK_SH` holders release.
2. Read all entries from partition (fresh read-only handle).
3. Apply substitutions, write temp file, `fsync(tmp)`.
4. `Evict(relPath)` — close the cached write FD in `JSONLWriter.files`, delete from `sync.Map`.
5. `rename(tmp, original)`, `fsync(parentDir)`.
6. `flock(LOCK_UN)` + `close(fd)`.
7. Next append: opens `.lock` → `LOCK_SH` (succeeds immediately) → `getOrCreate` reopens the JSONL file.

Between steps 1 and 6, any writer trying `flock(LOCK_SH)` blocks. After step 6, they proceed and `getOrCreate` opens the rewritten file.

#### Crash recovery

flock auto-releases on process death, so there are **no stale locks**. But the lock release doesn't tell us the rewrite state. Two options:

**Option A: Temp file convention (simple)**

On startup, scan for `*.compact.tmp` files:
- Temp exists, original exists → compaction didn't finish rename. Delete temp. Original is intact.
- Temp exists, original missing → impossible on a crash-safe filesystem (rename is atomic). Defensively: rename temp to original.
- No temp → clean state.

This is sufficient because:
- Before rename: temp is partial/complete, but original is untouched → safe to discard temp.
- After rename: temp is gone (or was the old content via `RENAME_EXCHANGE`), original has new content → already consistent.
- Bookkeeping (index rebuild, candidate moves, journal) is idempotent and can be re-derived.

**Option B: SQLite intent row (inspectable)**

Same as Approach 1 from previous analysis — `compaction_locks` table with `started/rewriting/completed` state. More inspectable, slightly more complex.

**Recommendation: Option A for v1.** The temp file convention is self-describing (no schema migration), and the bookkeeping steps are idempotent. Upgrade to Option B if debugging/observability needs arise.

#### Cross-process compatibility

Since flock is OS-level, this design naturally extends to a **separate compactor process** — the architectural direction the CANDIDATE_COMPACTION.md already hints at ("a dedicated compactor operates on log storage directly, not through the writer"). No code changes needed; the compactor just opens the same `.lock` files.

---

### Rejected strategies (for reference)

#### Strategy A: Per-partition `sync.RWMutex` in Store

In-process only. Doesn't survive crash. Doesn't extend to separate compactor process. ~50ns per append (faster than flock's ~3µs), but the speed difference is irrelevant at this scale.

#### Strategy B: OFD locks on JSONL files directly

Correct semantics, but locks attach to inode — invalidated by rename. Requires `golang.org/x/sys/unix` dependency for no benefit over flock + sidecar.

#### Strategy C: Store-wide lock

Too coarse — blocks all partitions during compaction of one.

#### Strategy D: SQLite advisory table

~1ms per lock operation. Too slow for append path. Useful only as a crash-recovery manifest (Option B above).

#### eBPF

Wrong abstraction level. Rejected.

---

### Required code changes

| Change | Scope | Est. lines |
|--------|-------|------------|
| `pkg/flock` — add `SharedLock`/`ExclusiveLock` helpers (new fds) | `pkg/flock/flock.go` | ~20 |
| `JSONLWriter.BuildRelPath` → public | `jsonlstore/jsonl.go` | 1 |
| `JSONLWriter.Evict(relPath)` — close FD + delete from `files` | `jsonlstore/jsonl.go` | ~10 |
| `Store.Append` — resolve path → flock(SH) → write → unlock | `store.go` | ~10 |
| `Store.Compact` — flock(EX) → rewrite → evict → rename → unlock | `store.go` | new method |
| `Store.recoverCompaction()` — scan for `*.compact.tmp` on startup | `store.go` | ~15 |
| Add `google/renameio/v2` dependency | `go.mod` | 1 |

### Rewrite protocol (final)

```
compact(partition relPath, resolutions []Resolution):
    lockPath := relPath + ".lock"

    fd := open(lockPath, O_CREATE|O_RDWR)
    flock(fd, LOCK_EX)                              // 1. exclusive — blocks writers
    defer { flock(fd, LOCK_UN); close(fd) }

    entries := readAll(relPath)                      // 2. fresh read handle
    rewritten := applySubstitutions(entries, resolutions)

    tmp := relPath + ".compact.tmp"
    renameio.WriteFile(tmp, rewritten, 0644)         // 3. write + fsync + rename
                                                     //    (renameio handles fsync discipline)

    rawStore.Evict(relPath)                          // 4. invalidate cached FD

    // 5. Bookkeeping (idempotent — safe to re-run after crash)
    metaStore.Rebuild(relPath, rewritten)
    moveCandidates(resolutions)
    journal.Append(resolutions)
```

### Crash recovery

```
Store.NewStore():
    ...existing init...

    tmpFiles := glob(dk_logs/**/*.compact.tmp)
    for _, tmp := range tmpFiles {
        os.Remove(tmp)                               // original is intact, discard partial rewrite
        log.Warn("removed orphaned compaction temp", "file", tmp)
    }
```

---

## 4. Invariants review

### Stated invariants — assessment

| Invariant | Valid? | Notes |
|-----------|--------|-------|
| Log entries replaced **only** through compaction | Yes | Consistent with append-only design. |
| Candidate references exactly one existing entry by ID | Yes | Sound. |
| Multiple candidates per entry are valid; compaction resolves all | Yes | Good. |
| Journal is append-only, never rewritten | Yes | Sound. |

### Missing invariants

| # | Missing invariant | Why it matters |
|---|-------------------|----------------|
| 4.1 | **A candidate preserves the original entry's partition key (day-bucket from `Timestamp`).** | If a candidate changes `Timestamp` to a different day, it would belong in a different partition, breaking in-place replacement. Either disallow cross-partition edits or define a move-on-compact semantic. |
| 4.2 | **After compaction the index reflects the new partition state.** | Tags/time-range could shift. Stale index means queries return wrong files. |
| 4.3 | **No concurrent compaction on the same partition.** | Two compactors rewriting the same file simultaneously would corrupt data. |
| 4.4 | **Appends to a partition under compaction are serialized.** | Otherwise data loss (see 3.4). |

---

## 5. Open questions — recommendations

### Index: extend or scan pending?

The doc asks whether to extend the index with a candidates table or scan pending directly.

**Recommendation: scan pending.** Rationale:
- Candidate volume is expected to be low (personal logging, edits are rare).
- Scanning `pending/` avoids coupling the compaction path to the index's schema.
- The index is file-level by design; adding per-entry tracking is a larger change.
- If candidate volume later becomes a concern, an index extension is a backwards-compatible addition.

However, the compactor still needs to locate the **original** entry's partition. This is separate from indexing candidates — see section 2.

### Conflict: all-at-once or partial?

The doc asks whether multiple candidates per entry must be resolved all at once.

**Recommendation: all-at-once.** Rationale:
- If partial resolution is allowed, subsequent compaction runs must re-scan the same partition for leftover candidates, making the process non-idempotent.
- All-at-once keeps the model simpler: each compaction run produces a clean partition with no remaining pending references.
- The invariant in the doc already says "compaction must resolve all" — elevate this from invariant to explicit API requirement.

---

## Summary

The design is directionally sound. The main gaps are:

1. **ID lookup** — no mechanism to find which file holds the original entry (High).
2. **FD cache invalidation** — rewriting a file underneath the writer loses the cached descriptor (High).
3. **Append/compaction race** — concurrent appends to a partition being rewritten will lose data (High).
4. **Crash recovery** — no way to detect or recover from a crash mid-compaction (High).
5. **Missing invariants** — cross-partition timestamp edits, index consistency, and concurrency guards.

All are solvable within the current architecture. The recommended approach is:

- Candidates carry the original's timestamp (no index change needed).
- **Per-partition `flock()` on sidecar `.lock` files** (Strategy E). Writers acquire `LOCK_SH` per append (~3–5µs), compactor acquires `LOCK_EX` (blocks until writers release). Provides both runtime coordination AND crash-safe auto-release — no in-process mutexes, no SQLite lock tables. Extends naturally to a separate compactor process.
- **Crash recovery:** Temp file convention (`*.compact.tmp`). On startup, scan and delete orphaned temps — original is always intact. Bookkeeping is idempotent.
- **Fsync discipline:** Use `google/renameio/v2` — handles temp write, fsync, rename, and directory fsync correctly.
- Mandatory all-at-once resolution per entry.
