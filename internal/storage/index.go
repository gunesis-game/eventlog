package storage

import (
	"github.com/gunesis-game/eventlog/internal/fieldschema"
	"github.com/gunesis-game/eventlog/internal/identity"
)

// Occurrence is the derived location of one committed Event occurrence.
type Occurrence struct {
	Sequence  uint64
	EventID   identity.UUID
	ChannelID identity.UUID
	Type      fieldschema.TypeKey
	segment   *segment
	offset    int64
	length    int64
}

// channelSnapshot is the latest persisted state of a logical Channel; fields is
// the canonical JSON of its field map.
type channelSnapshot struct {
	typ    fieldschema.TypeKey
	fields string
}

// index is the in-memory derived index: every committed occurrence in global
// sequence order, per-Channel occurrence lists, and the latest Channel
// snapshots. It is rebuilt from the segments on every start.
type index struct {
	occurrences []*Occurrence
	byChannel   map[identity.UUID][]*Occurrence
	channels    map[identity.UUID]channelSnapshot
}

func newIndex() *index {
	return &index{byChannel: make(map[identity.UUID][]*Occurrence), channels: make(map[identity.UUID]channelSnapshot)}
}

func (x *index) add(occurrence *Occurrence) {
	x.occurrences = append(x.occurrences, occurrence)
	x.byChannel[occurrence.ChannelID] = append(x.byChannel[occurrence.ChannelID], occurrence)
}

func (x *index) occurrence(channelID identity.UUID, position int) (*Occurrence, bool) {
	list := x.byChannel[channelID]
	if position >= len(list) {
		return nil, false
	}
	return list[position], true
}

// historyTypes returns the distinct Event types in a Channel's committed
// history, in first-occurrence order.
func (x *index) historyTypes(channelID identity.UUID) []fieldschema.TypeKey {
	seen := make(map[fieldschema.TypeKey]bool)
	var types []fieldschema.TypeKey
	for _, occurrence := range x.byChannel[channelID] {
		if !seen[occurrence.Type] {
			seen[occurrence.Type] = true
			types = append(types, occurrence.Type)
		}
	}
	return types
}

func (x *index) channel(id identity.UUID) (channelSnapshot, bool) {
	snapshot, ok := x.channels[id]
	return snapshot, ok
}

func (x *index) setChannel(id identity.UUID, snapshot channelSnapshot) {
	x.channels[id] = snapshot
}

// relocate repoints occurrences in compacted source segments at their place in
// the replacement segment.
func (x *index) relocate(bases map[*segment]int64, replacement *segment) {
	for _, occurrence := range x.occurrences {
		if base, ok := bases[occurrence.segment]; ok {
			occurrence.offset += base
			occurrence.segment = replacement
		}
	}
}

// persisted returns the generation-bound persisted form of the index.
func (x *index) persisted(generation, committedSequence uint64) indexFile {
	file := indexFile{FormatVersion: indexFormatVersion, Generation: generation, CommittedSequence: committedSequence, Entries: make([]indexFileEntry, 0, len(x.occurrences))}
	for _, occurrence := range x.occurrences {
		file.Entries = append(file.Entries, indexFileEntry{
			Sequence:  occurrence.Sequence,
			EventID:   occurrence.EventID.String(),
			ChannelID: occurrence.ChannelID.String(),
			Segment:   segmentFileName(occurrence.segment.id),
			Offset:    occurrence.offset,
			Length:    occurrence.length,
		})
	}
	return file
}
