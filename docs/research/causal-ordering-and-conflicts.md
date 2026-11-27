# Research: causal ordering and conflict detection across many devices

**Status:** Research note, 2026-11-27. It informs the multi-device decisions D-056 to D-059 and the proposed multi-device design; the current specification is owned by [requirements.md](../project/requirements.md) and [architecture.md](../project/architecture.md). Several claims below come from secondary sources (summaries, course notes, implementation libraries); they are marked where the primary text was not read directly.

## Question

Several devices, each running one eventlog process, write concurrently to the same Channels through shared storage (D-056). Readers must deliver occurrences in causal order (D-057), concurrent changes to the same logical Event must be reported as conflicts (D-058), and remote changes are applied to local live instances (D-059). Which mechanisms provide causal order and concurrency detection, and how do they behave as the number of devices grows large?

## Findings

### Vector clocks and version vectors

- A vector clock holds one counter per source of events. Comparing two clocks gives a partial order: one happened before the other, or they are concurrent. Concurrent means neither writer had seen the other's event, which is exactly the condition for a conflict.
- The cost is size: one entry is needed for each source of concurrency, so space grows linearly with the number of participants. A plain vector clock also assumes the set of participants is known.
- Riak's experience: if clients rather than servers get entries, each data item's clock grows with the number of clients that ever modified it.

### Variants that address growth

- **Dotted version vectors** keep a causal-past vector plus the most recent event as a separate "dot". Entries stay tied to a small set of replicas rather than to every client, while still detecting concurrency. They target a small set of servers mediating many clients.
- **Concise version vectors** allow a dynamic number of replicas.
- **Pruning** drops old entries. It is simple but can produce false merges, and ad-hoc pruning (for example in Dynamo) can compromise causality tracking.
- **Interval tree clocks** support fully decentralized creation and retirement of participants without global identifiers. The authors present them as generalizing version vectors and vector clocks, compact enough even for fixed membership, and a way to avoid aggressive pruning.
- **Plausible clocks** and **hash histories** use less space but can misreport concurrency: plausible clocks may order concurrent events, and hash histories are only statistically correct.
- No head-to-head benchmark of interval tree clocks against pruned vector clocks at high device counts was found.

### Causal delivery

- The standard blocking algorithm: a sender attaches its vector; a receiver holds an incoming message until every event in that vector has been delivered, then delivers it. A non-blocking alternative ships the causal past with each message. Buffering at the sender instead of the receiver is much less used.
- Blocking delivery holds back only events that depend on something not yet received. Unrelated events from other devices are not delayed, so one offline device stalls only what causally depends on it.

### Hybrid logical clocks

- Hybrid logical clocks (Kulkarni, Demirbas, Madeppa, Avva, Leone) combine physical and logical time. They capture happened-before like logical clocks while staying close to wall-clock time, and make consistent snapshots easy to identify.
- They give a total order that respects causality, but alone they cannot tell whether two events were concurrent.
- Implementations break ties deterministically by comparing physical time, then the logical counter, then a node identifier. That tie-break wording comes from implementation libraries; it was not verified against the paper's text.

## Implications for eventlog

- **Concurrency detection needs vectors (or an interval tree clock), not timestamps.** A hybrid logical clock alone can order events but cannot identify conflicts.
- **A deterministic current state needs a total order.** A hybrid logical clock with a device-identifier tie-break gives every device the same winner for a conflict without coordination.
- **Size must be bounded for large distribution.** Two choices fit eventlog:
  - Scope vectors per Channel, so they hold entries only for devices that have written to that Channel, rather than for every device that ever wrote anything.
  - Or adopt interval tree clocks when devices join and leave frequently.
- **Delivery follows the blocking causal algorithm per Channel.** A remote occurrence waits until its Channel dependencies are delivered; its device's own occurrences stay in their per-device order.
- **Pruning is unsafe for conflict detection.** A false merge would hide or invent conflicts, so it should not be used without a correctness argument.

## Sources

- [Dotted Version Vectors: Logical Clocks for Optimistic Replication (arXiv)](https://arxiv.org/abs/1011.5808)
- [Wikipedia: Version vector](https://en.wikipedia.org/wiki/Version_vector)
- [Wikipedia: Vector clock](https://wikipedia.com/wiki/Vector_clock)
- [Causality is Graphically Simple (arXiv)](https://arxiv.org/pdf/2012.09086)
- [ACM Queue: Why Logical Clocks are Easy](https://queue.acm.org/detail.cfm?id=2917756)
- [Rutgers: Logical clocks lecture notes](https://people.cs.rutgers.edu/pxk/417/notes/logical-clocks.html)
- [Riak: Why vector clocks are hard](https://riak.com/posts/technical/why-vector-clocks-are-hard/)
- [Interval Tree Clocks: A Logical Clock for Dynamic Systems](https://core.ac.uk/download/55634211.pdf)
- [Springer: Interval Tree Clocks](https://link.springer.com/chapter/10.1007/978-3-540-92221-6_18)
- [ECEASST: Towards logical clocks in P2P-based MMVEs (pruned clocks)](https://journal.ub.tu-berlin.de/eceasst/article/view/221)
- [EPFL: Causal broadcast lecture slides](https://dcl.epfl.ch/site/_media/education/da17-causalbroadcast.pdf)
- [Byzantine Reliable Broadcast with Causal Ordering (arXiv)](https://arxiv.org/pdf/2609.17074)
- [Causal broadcast buffering approaches (arXiv)](https://arxiv.org/pdf/2601.11487)
- [Breaking the Scalability Barrier of Causal Broadcast for Large and Dynamic Systems (arXiv)](https://arxiv.org/pdf/1805.05201)
- [Logical Physical Clocks (Kulkarni et al.)](https://www.cse.buffalo.edu//~demirbas/publications/hlc.pdf)
- [Achieving Causality with Physical Clocks (arXiv)](https://arxiv.org/pdf/2104.15099v2)
- [crdt-kit: hybrid logical clock implementation](https://docs.rs/crdt-kit/0.5.0/src/crdt_kit/clock.rs.html)
- [silk-graph: hybrid logical clock implementation](https://docs.rs/silk-graph/0.1.3/src/silk/clock.rs.html)
