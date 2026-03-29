# Candidate Compaction

Entry-level replacement for an append-only log store. 
A **candidate** is a proposed replacement for an existing entry, stored separately. 
**Compaction** substitutes originals with winning candidates by rewriting source files.

## File layout

Logical partitioning — storage backend resolves to concrete locations (filesystem, object store, etc.):

```
logs/{partition-X}                  source of truth, append-only
candidates/
  pending/{partition-X}             unresolved
  applied/{partition-X}             winners — purgeable
  dismissed/{partition-X}           losers — purgeable
  journal/{journal-partition}       audit trail — durable
```

`{partition}` is time-bucketed and should have same time-start-time-end as logs/candidates.


## How it works

A candidate is a **full copy** of an existing entry with edits applied — same `Entry` struct, same ID as the original.
The storage layer matches candidates to originals by ID.

```
write candidate  →  append to pending
list collisions  →  entries in logs that have unresolved candidates
compact          →  caller resolves collisions, storage executes substitution
```

## Compaction

1. For each applied candidate: **replace** the matching entry in `logs` by ID, move candidate `pending → applied`.
2. For each dismissed candidate: move candidate `pending → dismissed`.
3. Rewrite affected partitions atomically.
4. Append resolution to `journal`.

### Architectural tension

The log writer is append-only. Compaction rewrites partitions — different access pattern, different failure modes. 
These should stay separate: a dedicated compactor operates on log storage directly, not through the writer.
The store facade coordinates so compaction and appends don't race on the same partition.

## Invariants

- Log entries are replaced **only** through compaction.
- A candidate references exactly one existing entry (matched by ID).
- Multiple candidates per entry are valid — compaction must resolve all of them (apply one, dismiss rest).
- `journal/` is append-only, never rewritten.

## Open

- **Index**: extend the existing index with a candidates table, or scan pending directly? Scanning is simpler and correct until candidate volume makes it slow.
- **Conflict**: multiple candidates for same entry — require caller to resolve all at once, or allow partial?
