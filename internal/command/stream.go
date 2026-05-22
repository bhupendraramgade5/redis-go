package command

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/codecrafters-io/redis-starter-go/internal/resp"
	"github.com/codecrafters-io/redis-starter-go/internal/store"
)

type XADDCommand struct{ Store *store.DataStore }

func (xadd XADDCommand) Execute(args []string) string {
	key := args[1]
	id := args[2]

	if (len(args)-3)%2 != 0 {
		return "-ERR wrong number of arguments\r\n"
	}

	xadd.Store.Lock()
	defer xadd.Store.Unlock()

	stream, ok := xadd.Store.DataStream[key]
	if !ok {
		stream = &store.Stream{}
		xadd.Store.DataStream[key] = stream
	}

	var ms, seq int64
	var err error

	if id == "*" {
		ms = time.Now().UnixMilli()
		seq = generateSeq(stream, ms)
	} else if strings.HasSuffix(id, "-*") {
		parts := strings.Split(id, "-")
		ms, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return "-ERR Invalid stream ID\r\n"
		}
		seq = generateSeq(stream, ms)
	} else {
		ms, seq, err = parseID(id)
		if err != nil {
			return "-ERR Invalid stream ID\r\n"
		}
	}

	if ms == 0 && seq == 0 {
		return "-ERR The ID specified in XADD must be greater than 0-0\r\n"
	}

	if len(stream.Entries) > 0 {
		if ms < stream.TopIDTime {
			return "-ERR The ID specified in XADD is equal or smaller than the target stream top item\r\n"
		}
		if ms == stream.TopIDTime && seq <= stream.TopIDSeq {
			return "-ERR The ID specified in XADD is equal or smaller than the target stream top item\r\n"
		}
	}

	finalID := fmt.Sprintf("%d-%d", ms, seq)
	fields := make([]string, 0)
	for i := 3; i < len(args); i++ {
		fields = append(fields, args[i])
	}

	entry := store.StreamEntry{
		ID:     finalID,
		Fields: fields,
	}

	stream.Entries = append(stream.Entries, entry)
	stream.TopIDTime = ms
	stream.TopIDSeq = seq

	waiters := append([]chan struct{}(nil), xadd.Store.StreamWaiters[key]...)
	for _, ch := range waiters {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
	xadd.Store.KeyVersion[key]++
	delete(xadd.Store.StreamWaiters, key)

	return resp.EncodeBulkString(finalID)
}

func generateSeq(stream *store.Stream, ms int64) int64 {
	if len(stream.Entries) == 0 {
		if ms == 0 {
			return 1
		}
		return 0
	}
	if ms == stream.TopIDTime {
		return stream.TopIDSeq + 1
	}
	if ms == 0 {
		return 1
	}
	return 0
}

func parseID(id string) (int64, int64, error) {
	parts := strings.Split(id, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid id")
	}
	ms, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, 0, err
	}
	seq, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, 0, err
	}
	return ms, seq, nil
}

type XRANGECommand struct{ Store *store.DataStore }

func (xrange XRANGECommand) Execute(args []string) string {
	key := args[1]
	start := normalizeStartID(args[2])
	end := normalizeEndID(args[3])

	stream, ok := xrange.Store.DataStream[key]
	if !ok {
		return "*0\r\n"
	}

	var result []store.StreamEntry
	for _, entry := range stream.Entries {
		if compareIDs(entry.ID, start) >= 0 && compareIDs(entry.ID, end) <= 0 {
			result = append(result, entry)
		}
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("*%d\r\n", len(result)))
	for _, entry := range result {
		builder.WriteString("*2\r\n")
		builder.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(entry.ID), entry.ID))
		builder.WriteString(fmt.Sprintf("*%d\r\n", len(entry.Fields)))
		for _, val := range entry.Fields {
			builder.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(val), val))
		}
	}
	return builder.String()
}

func normalizeStartID(id string) string {
	if id == "-" {
		return "0-0"
	}
	if !strings.Contains(id, "-") {
		return id + "-0"
	}
	return id
}

func normalizeEndID(id string) string {
	if id == "+" {
		return fmt.Sprintf("%d-%d", int64(math.MaxInt64), int64(math.MaxInt64))
	}
	if !strings.Contains(id, "-") {
		return id + "-" + strconv.FormatInt(math.MaxInt64, 10)
	}
	return id
}

func compareIDs(id1, id2 string) int {
	ms1, seq1, _ := parseID(id1)
	ms2, seq2, _ := parseID(id2)
	if ms1 < ms2 {
		return -1
	} else if ms1 > ms2 {
		return 1
	}
	if seq1 < seq2 {
		return -1
	} else if seq1 > seq2 {
		return 1
	}
	return 0
}

type XREADCommand struct{ Store *store.DataStore }

func (cmd XREADCommand) Execute(args []string) string {
	if len(args) < 4 {
		return "-ERR wrong number of arguments\r\n"
	}
	i := 1
	block := false
	var timeout time.Duration

	if strings.ToUpper(args[i]) == "BLOCK" {
		block = true
		ms, _ := strconv.Atoi(args[i+1])
		if ms == 0 {
			timeout = 0 // special case: infinite
		} else {
			timeout = time.Duration(ms) * time.Millisecond
		}
		i += 2
	}

	if strings.ToUpper(args[i]) != "STREAMS" {
		return "-ERR syntax error\r\n"
	}
	i++

	total := len(args) - i
	if total%2 != 0 {
		return "-ERR syntax error\r\n"
	}

	n := total / 2
	keys := args[i : i+n]
	ids := args[i+n : i+2*n]

	for i := 0; i < n; i++ {
		if ids[i] == "$" {
			stream, ok := cmd.Store.DataStream[keys[i]]
			if ok && len(stream.Entries) > 0 {
				ids[i] = fmt.Sprintf("%d-%d", stream.TopIDTime, stream.TopIDSeq)
			} else {
				ids[i] = "0-0"
			}
		}
	}

	var streamsData []streamResult
	for i := 0; i < n; i++ {
		entries := xreadfunc(cmd.Store, keys[i], ids[i])
		if len(entries) > 0 {
			streamsData = append(streamsData, streamResult{key: keys[i], entries: entries})
		}
	}

	if len(streamsData) > 0 || !block {
		return encodeMultiStream(streamsData)
	}

	ch := make(chan struct{}, 1)

	cmd.Store.Lock()
	for _, key := range keys {
		cmd.Store.StreamWaiters[key] = append(cmd.Store.StreamWaiters[key], ch)
	}
	cmd.Store.Unlock()

	if timeout == 0 {
		// infinite block
		<-ch
		var newData []streamResult
		for i := 0; i < n; i++ {
			entries := xreadfunc(cmd.Store, keys[i], ids[i])
			if len(entries) > 0 {
				newData = append(newData, streamResult{keys[i], entries})
			}
		}
		return encodeMultiStream(newData)
	}

	select {
	case <-ch:
		var newData []streamResult
		for i := 0; i < n; i++ {
			entries := xreadfunc(cmd.Store, keys[i], ids[i])
			if len(entries) > 0 {
				newData = append(newData, streamResult{keys[i], entries})
			}
		}
		return encodeMultiStream(newData)
	case <-time.After(timeout):
		cmd.Store.Lock()
		for _, key := range keys {
			waiters := cmd.Store.StreamWaiters[key]
			var newList []chan struct{}
			for _, w := range waiters {
				if w != ch {
					newList = append(newList, w)
				}
			}
			cmd.Store.StreamWaiters[key] = newList
		}
		cmd.Store.Unlock()
		return "*-1\r\n"
	}
}

// streamResult pairs a stream key with the entries returned for it. It replaces
// the anonymous struct used inline in the original code; a named type is
// clearer once the logic is split across helpers.
type streamResult struct {
	key     string
	entries []store.StreamEntry
}

func xreadfunc(s *store.DataStore, key string, lastID string) []store.StreamEntry {
	stream, ok := s.DataStream[key]
	if !ok {
		return nil
	}
	var result []store.StreamEntry
	for _, e := range stream.Entries {
		if compareIDs(e.ID, lastID) > 0 {
			result = append(result, e)
		}
	}
	return result
}

func encodeMultiStream(data []streamResult) string {
	if len(data) == 0 {
		return "*0\r\n"
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("*%d\r\n", len(data)))
	for _, stream := range data {
		b.WriteString("*2\r\n")
		b.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(stream.key), stream.key))
		b.WriteString(fmt.Sprintf("*%d\r\n", len(stream.entries)))
		for _, e := range stream.entries {
			b.WriteString("*2\r\n")
			b.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(e.ID), e.ID))
			b.WriteString(fmt.Sprintf("*%d\r\n", len(e.Fields)))
			for _, f := range e.Fields {
				b.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(f), f))
			}
		}
	}
	return b.String()
}

type TYPECommand struct{ Store *store.DataStore }

func (t TYPECommand) Execute(args []string) string {
	key := args[1]
	if _, ok := t.Store.KV[key]; ok {
		return "+string\r\n"
	}
	if _, ok := t.Store.Lists[key]; ok {
		return "+list\r\n"
	}
	if _, ok := t.Store.DataStream[key]; ok {
		return "+stream\r\n"
	}
	return "+none\r\n"
}
