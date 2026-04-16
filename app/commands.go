package main

import (
	// "fmt"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
)

// May be there is a method which doesnt require the use of arity in future
// But to write implementation of that method we were still dependent on the
// Arity function
// thus by segregatin the interface we can now directky use the methods that we actually
// need, and saving efforts in writing methods that are unneccesary

var internalmap = make(map[string]internalState)

type internalState struct {
	value     string
	expiresAt time.Time
}

type variables struct {
	listleft  []string
	listright []string
}

type DataStore struct {
	KV         map[string]internalState
	Lists      map[string]variables
	ListWaiters    map[string][]chan []string
	StreamWaiters map[string][]chan struct{}
	DataStream map[string]*Stream
	syncmut    sync.Mutex
}

type StreamEntry struct {
	ID     string
	Fields [] string
	Time   int64
	Seq    int64
}

type Stream struct {
	Entries    []StreamEntry
	TopId_time int64
	TopId_seq  int64
}

func NewStore() *DataStore {
	return &DataStore{
		KV:         make(map[string]internalState),
		Lists:      make(map[string]variables),
		ListWaiters:    make(map[string][]chan []string),
		StreamWaiters: make(map[string][]chan struct{}),
		DataStream: make(map[string]*Stream),
	}
}

type Command interface {
	Execute(args []string) string
}

type ArityChecker interface {
	Arity() int
}

type PingCommand struct{}

type EchoCommand struct{}
type SetCommand struct {
	Store *DataStore
}
type GetCommand struct {
	Store *DataStore
}
type RpushCommand struct {
	Store *DataStore
}
type LRangeCommand struct {
	Store *DataStore
}
type LPushCommand struct {
	Store *DataStore
}
type LLenCommand struct {
	Store *DataStore
}

type BLPopCommand struct {
	Store *DataStore
}

type LPopCommand struct {
	Store *DataStore
}
type TYPECommand struct {
	Store *DataStore
}

type XADDCommand struct {
	Store *DataStore
}

type XRANGECommand struct {
	Store *DataStore
}
type XREADCommand struct {
	Store *DataStore
}

func NewRegistry(store *DataStore) map[string]Command {
	return map[string]Command{
		"PING":   PingCommand{},
		"ECHO":   EchoCommand{},
		"SET":    SetCommand{Store: store},
		"GET":    GetCommand{Store: store},
		"RPUSH":  RpushCommand{Store: store},
		"LRANGE": LRangeCommand{Store: store},
		"LPUSH":  LPushCommand{Store: store},
		"LLEN":   LLenCommand{Store: store},
		"LPOP":   LPopCommand{Store: store},
		"BLPOP":  BLPopCommand{Store: store},
		"TYPE":   TYPECommand{Store: store},
		"XADD":   XADDCommand{Store: store},
		"XRANGE": XRANGECommand{Store: store},
		"XREAD":   XREADCommand{Store: store}, // For simplicity, using XRANGE implementation for XREAD
	}
}

func (ping PingCommand) Execute(args []string) string {
	return "+PONG\r\n"
}

func (echo EchoCommand) Execute(args []string) string {
	return encodeBulkString(args[1])
}
func (echo EchoCommand) Arity() int {
	return 2
}

// var internalmap = make(map[string]internalState)

// type internalState struct {
// 	value     string
// 	expiresAt time.Time
// }

func (set SetCommand) Execute(args []string) string {
	key := args[1]
	value := args[2]

	var expiresAt time.Time

	for i := 3; i < len(args); i++ {
		switch strings.ToUpper(args[i]) {

		case "EX":
			seconds, _ := strconv.Atoi(args[i+1])
			expiresAt = time.Now().Add(time.Duration(seconds) * time.Second)
			i++

		case "PX":
			ms, _ := strconv.Atoi(args[i+1])
			expiresAt = time.Now().Add(time.Duration(ms) * time.Millisecond)
			i++
		}
	}

	set.Store.KV[key] = internalState{
		value:     value,
		expiresAt: expiresAt,
	}

	return "+OK\r\n"
}

func (get GetCommand) Execute(args []string) string {
	key := args[1]

	state, ok := get.Store.KV[key]
	if !ok {
		return "$-1\r\n"
	}

	if !state.expiresAt.IsZero() && time.Now().After(state.expiresAt) {
		delete(internalmap, key)
		return "$-1\r\n"
	}

	return encodeBulkString(state.value)
}

// var Rpushmap = make(map[string]variables)

// type variables struct {
// 	listright []string
// }

func (rpush RpushCommand) Execute(args []string) string {
	key := args[1]
	rpush.Store.syncmut.Lock()

	waiters := rpush.Store.ListWaiters[key]

	if len(waiters) > 0 {
		ch := waiters[0]
		rpush.Store.ListWaiters[key] = waiters[1:]

		val := args[2]

		rpush.Store.syncmut.Unlock()

		ch <- []string{key, val}

		return fmt.Sprintf(":%d\r\n", 1)
	}

	// var temp variables
	temp, ok := rpush.Store.Lists[key]

	if !ok {
		temp = variables{}
	}

	for i := 2; i < len(args); i++ {
		temp.listright = append(temp.listright, args[i])
	}
	rpush.Store.Lists[key] = temp
	total := len(temp.listleft) + len(temp.listright)

	rpush.Store.syncmut.Unlock()

	response := fmt.Sprintf(":%d\r\n", total)
	return response
}

func (lpush LPushCommand) Execute(args []string) string {
	key := args[1]
	lpush.Store.syncmut.Lock()

	waiters := lpush.Store.ListWaiters[key]

	if len(waiters) > 0 {
		ch := waiters[0]
		lpush.Store.ListWaiters[key] = waiters[1:]

		val := args[2]

		lpush.Store.syncmut.Unlock()

		ch <- []string{key, val}

		return fmt.Sprintf(":%d\r\n", 1)
	}

	temp, ok := lpush.Store.Lists[key]

	if !ok {
		temp = variables{}
	}

	for i := 2; i < len(args); i++ {
		temp.listleft = append(temp.listleft, args[i])
	}

	lpush.Store.Lists[key] = temp
	total := len(temp.listleft) + len(temp.listright)

	lpush.Store.syncmut.Unlock()
	response := fmt.Sprintf(":%d\r\n", total)
	return response
}

func (lpop LPopCommand) Execute(args []string) string {
	key := args[1]
	var lft int = 1
	if len(args) > 2 {
		lft, _ = strconv.Atoi(args[2])
	}

	temp, ok := lpop.Store.Lists[key]

	if !ok {
		return "$-1\r\n"
		// temp = variables{}
	}

	// var popkey []string
	popkey := make([]string, 0, lft)
	for i := 0; i < lft; i++ {
		if len(temp.listleft) != 0 {
			popkey = append(popkey, temp.listleft[len(temp.listleft)-1])
			temp.listleft = temp.listleft[0 : len(temp.listleft)-1]
		} else if len(temp.listright) != 0 {
			popkey = append(popkey, temp.listright[0])
			temp.listright = temp.listright[1:]
		} else if len(popkey) == 0 {
			return "$-1\r\n"
		} else {
			break
		}
	}

	if len(temp.listleft) == 0 && len(temp.listright) == 0 {
		delete(lpop.Store.Lists, key)
	} else {
		lpop.Store.Lists[key] = temp
	}

	var builder strings.Builder
	if len(popkey) == 1 {
		return fmt.Sprintf("$%d\r\n%s\r\n", len(popkey[0]), popkey[0])
	} else {
		builder.WriteString(fmt.Sprintf("*%d\r\n", len(popkey)))
	}
	// response:=fmt.Sprintf()
	for i := 0; i < len(popkey); i++ {
		val := popkey[i]
		builder.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(val), val))
	}
	return builder.String()
	// return fmt.Sprintf("$%d\r\n%s\r\n", len(popkey),popkey)
}

func (blpop BLPopCommand) Execute(args []string) string {
	key := args[1]

	timeout := 0.0
	if len(args) > 2 {
		timeout, _ = strconv.ParseFloat(args[2], 64)
	}

	blpop.Store.syncmut.Lock()

	temp, ok := blpop.Store.Lists[key]

	// immediate pop
	if ok && (len(temp.listleft) != 0 || len(temp.listright) != 0) {
		var val string

		if len(temp.listleft) != 0 {
			val = temp.listleft[len(temp.listleft)-1]
			temp.listleft = temp.listleft[:len(temp.listleft)-1]
		} else {
			val = temp.listright[0]
			temp.listright = temp.listright[1:]
		}

		blpop.Store.Lists[key] = temp
		blpop.Store.syncmut.Unlock()

		return encodeArray([]string{key, val})
	}

	ch := make(chan []string, 1)
	blpop.Store.ListWaiters[key] = append(blpop.Store.ListWaiters[key], ch)

	blpop.Store.syncmut.Unlock()

	// Infinite wait
	if timeout == 0 {
		result := <-ch
		return encodeArray(result)
	}

	// Timeout wait
	select {
	case result := <-ch:
		return encodeArray(result)

	case <-time.After(time.Duration(timeout * float64(time.Second))):

		// REMOVE WAITER
		blpop.Store.syncmut.Lock()

		waiters := blpop.Store.ListWaiters[key]
		for i, w := range waiters {
			if w == ch {
				blpop.Store.ListWaiters[key] =
					append(waiters[:i], waiters[i+1:]...)
				break
			}
		}

		blpop.Store.syncmut.Unlock()

		return "*-1\r\n"
	}
}

func encodeArray(input []string) string {
	var builder strings.Builder
	if len(input) == 0 {
		return "*-1\r\n"
	}

	builder.WriteString(fmt.Sprintf("*%d\r\n", len(input)))

	// response:=fmt.Sprintf()
	for i := 0; i < len(input); i++ {
		val := input[i]
		builder.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(val), val))
	}
	return builder.String()
}

func (llen LLenCommand) Execute(args []string) string {
	key := args[1]
	temp, ok := llen.Store.Lists[key]

	if !ok {
		temp = variables{}
	}
	total := len(temp.listleft) + len(temp.listright)

	response := fmt.Sprintf(":%d\r\n", total)
	return response
}

func (lrange LRangeCommand) Execute(args []string) string {
	key := args[1]
	lft, _ := strconv.Atoi(args[2])
	rgt, _ := strconv.Atoi(args[3])

	//Map Records : assignment and error handling
	temp, ok := lrange.Store.Lists[key]
	if !ok {
		return "*0\r\n"
	}

	combined := make([]string, 0)
	for i := len(temp.listleft) - 1; i >= 0; i-- {
		combined = append(combined, temp.listleft[i])
	}
	combined = append(combined, temp.listright...)

	size := len(combined)
	// negative indices handling
	if lft < 0 {
		lft = size + lft
	}
	if rgt < 0 {
		rgt = size + rgt
	}
	if lft < 0 {
		lft = 0
	}
	if rgt < 0 {
		return "*0\r\n"
	}
	if rgt >= size {
		rgt = size - 1
	}

	if lft > rgt || lft >= size {
		return "*0\r\n"
	}

	var builder strings.Builder

	start := lft
	end := min(rgt, len(combined)-1)
	builder.WriteString(fmt.Sprintf("*%d\r\n", end-start+1))

	// response:=fmt.Sprintf()
	for i := start; i <= end; i++ {
		val := combined[i]
		builder.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(val), val))
	}
	return builder.String()
}

func (type_ TYPECommand) Execute(args []string) string {
	key := args[1]

	if _, ok := type_.Store.KV[key]; ok {
		return "+string\r\n"
	}

	if _, ok := type_.Store.Lists[key]; ok {
		return "+list\r\n"
	}

	if _, ok := type_.Store.DataStream[key]; ok {
		return "+stream\r\n"
	}

	return "+none\r\n"
}

func (xadd XADDCommand) Execute(args []string) string {
	key := args[1]
	id := args[2]

	if (len(args)-3)%2 != 0 {
		return "-ERR wrong number of arguments\r\n"
	}

	xadd.Store.syncmut.Lock()
	defer xadd.Store.syncmut.Unlock()

	stream, ok := xadd.Store.DataStream[key]

	if !ok {
		stream = &Stream{}
		xadd.Store.DataStream[key] = stream
	}

	var ms, seq int64
	var err error

	if id == "*" {
		// ms = currentTimeMillis()
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

		if ms < stream.TopId_time {
			return "-ERR The ID specified in XADD is equal or smaller than the target stream top item\r\n"
		}

		if ms == stream.TopId_time && seq <= stream.TopId_seq {
			return "-ERR The ID specified in XADD is equal or smaller than the target stream top item\r\n"
		}
	}

	finalID := fmt.Sprintf("%d-%d", ms, seq)
	fields := make([]string, 0)

	for i := 3; i < len(args); i++ {
		// fields[args[i]] = args[i+1]
		fields = append(fields, args[i])
	}

	entry := StreamEntry{
		ID:     finalID,
		Fields: fields,
	}

	stream.Entries = append(stream.Entries, entry)
	stream.TopId_time = ms
	stream.TopId_seq = seq


	waiters := append([]chan struct{}(nil), xadd.Store.StreamWaiters[key]...)

	for _, ch := range waiters {
		select {
		case ch <- struct{}{}:
		default:
		}
	}

	delete(xadd.Store.StreamWaiters, key)

	return encodeBulkString(finalID)
}

func generateSeq(stream *Stream, ms int64) int64 {
	if len(stream.Entries) == 0 {
		if ms == 0 {
			return 1
		}
		return 0
	}

	if ms == stream.TopId_time {
		return stream.TopId_seq + 1
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

func (xrange XRANGECommand) Execute(args []string) string {
	key := args[1]
	start := normalizeStartID(args[2])
	end := normalizeEndID(args[3])

	stream, ok := xrange.Store.DataStream[key]
	if !ok {
		return "*0\r\n"
	}

	var result []StreamEntry

	for _, entry := range stream.Entries {
		if compareIDs(entry.ID, start) >= 0 &&
			compareIDs(entry.ID, end) <= 0 {
			result = append(result, entry)
		}
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("*%d\r\n", len(result)))

	for _, entry := range result {
		builder.WriteString("*2\r\n")

		// ID
		builder.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(entry.ID), entry.ID))

		// Fields
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
		return fmt.Sprintf("%d-%d", math.MaxInt64, math.MaxInt64)
	}
	if !strings.Contains(id, "-") {
		return id + "-" + strconv.FormatInt(math.MaxInt64, 10) // max seq
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
	} else {
		if seq1 < seq2 {
			return -1
		} else if seq1 > seq2 {
			return 1
		} else {
			return 0
		}
	}
}

// func (xread XREADCommand) Execute(args []string) string {
// 	// For simplicity, using XRANGE implementation for XREAD

// 	key := args[2]
//     id := args[3]

//     entries := xreadfunc(xread.Store, key, id)
// 	if len(entries) == 0 {
// 		return "*0\r\n"
// 	}
//     return encodeXRead(key, entries)
// 	// return (XRANGECommand{Store: xread.Store}).Execute(args)
// }

func (cmd XREADCommand) Execute(args []string) string {

	if len(args) < 4 {
		return "-ERR wrong number of arguments\r\n"
	}
	i:=1
	block:=false
	var timeout time.Duration

	if strings.ToUpper(args[i]) == "BLOCK" {
		block=true
		ms, _:=strconv.Atoi(args[i+1])

		if ms == 0 {
			timeout = 0 // special case: infinite
		} else {
			timeout = time.Duration(ms) * time.Millisecond
		}
		// timeout = time.Duration(ms)*time.Millisecond
		i+=2
	}

	if strings.ToUpper(args[i]) != "STREAMS" {
		return "-ERR syntax error\r\n"
	}
	i++

	// total := len(args) - 2
	total := len(args) - i

	if total%2 != 0 {
		return "-ERR syntax error\r\n"
	}

	n := total / 2

	keys := args[i : i+n]
	ids  := args[i+n : i+2*n]

	for i := 0; i < n; i++ {
		if ids[i] == "$" {
			stream, ok := cmd.Store.DataStream[keys[i]]
			if ok && len(stream.Entries) > 0 {
				ids[i] = fmt.Sprintf("%d-%d", stream.TopId_time, stream.TopId_seq)
			} else {
				ids[i] = "0-0"
			}
		}
	}
	var streamsData []struct {
		key     string
		entries []StreamEntry
	}

	for i := 0; i < n; i++ {
		entries := xreadfunc(cmd.Store, keys[i], ids[i])

		// 🔥 DEBUG HERE
		// fmt.Println("Key:", keys[i], "Entries:", entries)

		if len(entries) > 0 {
			streamsData = append(streamsData, struct {
				key     string
				entries []StreamEntry
			}{
				key:     keys[i],
				entries: entries,
			})
		}
	}

	if len(streamsData) > 0 || !block {
		return encodeMultiStream(streamsData)
	}

	ch := make(chan struct{}, 1)

	cmd.Store.syncmut.Lock()
	for _, key := range keys {
		cmd.Store.StreamWaiters[key] = append(cmd.Store.StreamWaiters[key], ch)
	}
	cmd.Store.syncmut.Unlock()
	if timeout == 0 {
		// infinite block
		<-ch

		// re-read after wakeup
		var newData []struct {
			key     string
			entries []StreamEntry
		}

		for i := 0; i < n; i++ {
			entries := xreadfunc(cmd.Store, keys[i], ids[i])
			if len(entries) > 0 {
				newData = append(newData, struct {
					key     string
					entries []StreamEntry
				}{keys[i], entries})
			}
		}
		return encodeMultiStream(newData)
	}else {	
		select {
		case <-ch:
			// re-run read after wakeup
			var newData []struct {
				key     string
				entries []StreamEntry
			}

			for i := 0; i < n; i++ {
				entries := xreadfunc(cmd.Store, keys[i], ids[i])
				if len(entries) > 0 {
					newData = append(newData, struct {
						key     string
						entries []StreamEntry
					}{keys[i], entries})
				}
			}

			return encodeMultiStream(newData)
			case <-time.After(timeout):

			cmd.Store.syncmut.Lock()
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
			cmd.Store.syncmut.Unlock()
			return "*-1\r\n"
		}
	}

	// fmt.Println("RAW RESP:")
	// fmt.Println(encodeMultiStream(streamsData))

	// return encodeMultiStream(streamsData)
}

func xreadfunc(store *DataStore, key string, lastID string) []StreamEntry {
	stream, ok := store.DataStream[key]
	if !ok {
		return nil
	}

	var result []StreamEntry

	for _, e := range stream.Entries {
		if compareIDs(e.ID, lastID) > 0 { // important
			result = append(result, e)
		}
	}
	return result
}

func encodeXRead(key string, entries []StreamEntry) string {
    if len(entries) == 0 {
        return "*0\r\n"
    }

    var b strings.Builder

    // 1 stream
    b.WriteString("*1\r\n")

    // [key, entries]
    b.WriteString("*2\r\n")

    // key
    b.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(key), key))

    // entries array
    b.WriteString(fmt.Sprintf("*%d\r\n", len(entries)))

    for _, e := range entries {
        b.WriteString("*2\r\n")

        // ID
        b.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(e.ID), e.ID))

        // fields
        b.WriteString(fmt.Sprintf("*%d\r\n", len(e.Fields)))

        for _, f := range e.Fields {
            b.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(f), f))
        }
    }

    return b.String()
}


func encodeMultiStream(data []struct {
	key     string
	entries []StreamEntry
}) string {

	if len(data) == 0 {
		return "*0\r\n"
	}

	var b strings.Builder

	// outer array (streams)
	b.WriteString(fmt.Sprintf("*%d\r\n", len(data)))

	for _, stream := range data {
		b.WriteString("*2\r\n")

		// key
		b.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(stream.key), stream.key))

		// entries array
		b.WriteString(fmt.Sprintf("*%d\r\n", len(stream.entries)))

		for _, e := range stream.entries {
			b.WriteString("*2\r\n")

			// ID
			b.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(e.ID), e.ID))

			// fields array
			b.WriteString(fmt.Sprintf("*%d\r\n", len(e.Fields)))

			for _, f := range e.Fields {
				b.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(f), f))
			}
		}
	}

	return b.String()
}



func handleCommand(registry map[string]Command, args []string) string {
	if len(args) == 0 {
		return "-ERR unknown command\r\n"
	}
	command := strings.ToUpper(args[0])
	handler := registry[command]

	if arityCmd, ok := handler.(ArityChecker); ok {
		if len(args) != arityCmd.Arity() {
			return "-ERR wrong number of arguments\r\n"
		}
	}
	return handler.Execute(args)
}
