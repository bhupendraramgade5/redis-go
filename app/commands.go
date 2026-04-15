package main

import (
	// "fmt"
	"fmt"
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
	listleft []string
	listright []string
}

type DataStore struct {
	KV    map[string]internalState
	Lists map[string]variables
	Waiters map[string][]chan []string
	DataStream map[string]*Stream
	syncmut sync.Mutex

}

type StreamEntry struct {
	ID string
	Fields map[string]string
}


type Stream struct {
	Entries []StreamEntry
	TopId_time int64
	TopId_seq int64
}

func NewStore() *DataStore {
	return &DataStore{
		KV:    make(map[string]internalState),
		Lists: make(map[string]variables),
		Waiters: make(map[string][]chan []string),
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
type LLenCommand struct{
	Store *DataStore
}

type BLPopCommand struct{
	Store * DataStore
}

type LPopCommand struct{
	Store *DataStore
}
type TYPECommand struct{
	Store *DataStore
}

type XADDCommand struct {
	Store *DataStore
}

func NewRegistry(store *DataStore) map[string]Command {
	return map[string]Command{
		"PING":  PingCommand{},
		"ECHO":  EchoCommand{},
		"SET":   SetCommand{Store: store},
		"GET":   GetCommand{Store: store},
		"RPUSH": RpushCommand{Store: store},
		"LRANGE": LRangeCommand{Store: store},
		"LPUSH": LPushCommand{Store: store},
		"LLEN" : LLenCommand{Store : store},
		"LPOP" :  LPopCommand{Store : store},
		"BLPOP" : BLPopCommand{Store : store},
		"TYPE" : TYPECommand{Store : store},
		"XADD" : XADDCommand{Store : store},
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

	waiters := rpush.Store.Waiters[key]

	if len(waiters) > 0 {
		ch := waiters[0]
		rpush.Store.Waiters[key] = waiters[1:]

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
	key:=args[1]
	lpush.Store.syncmut.Lock()

	waiters := lpush.Store.Waiters[key]

	if len(waiters) > 0 {
		ch := waiters[0]
		lpush.Store.Waiters[key] = waiters[1:]

		val := args[2]

		lpush.Store.syncmut.Unlock()

		ch <- []string{key, val}

		return fmt.Sprintf(":%d\r\n", 1)
	}

	temp, ok :=lpush.Store.Lists[key]


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
	key:=args[1]
	var lft int =1
	if len(args)>2 {
		lft, _=strconv.Atoi(args[2])
	}
	
	temp, ok :=lpop.Store.Lists[key]

	if !ok {
		return "$-1\r\n"
		// temp = variables{}
	}

	// var popkey []string
	popkey := make([]string, 0, lft)
	for i := 0; i < lft; i++ {
		if len(temp.listleft)!=0 {
			popkey= append(popkey, temp.listleft[len(temp.listleft)-1])
			temp.listleft=temp.listleft[0:len(temp.listleft)-1]
		}else if len(temp.listright)!=0 {
			popkey=append(popkey, temp.listright[0])
			temp.listright=temp.listright[1:]
		}else if len(popkey)==0{
			return "$-1\r\n"
		}else{
			break
		}
	}

	if len(temp.listleft) == 0 && len(temp.listright) == 0 {
		delete(lpop.Store.Lists, key)
	} else {
		lpop.Store.Lists[key] = temp
	}

	var builder strings.Builder
	if len(popkey)==1{
		return fmt.Sprintf("$%d\r\n%s\r\n", len(popkey[0]),popkey[0])
	}else {
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
	blpop.Store.Waiters[key] = append(blpop.Store.Waiters[key], ch)

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

		waiters := blpop.Store.Waiters[key]
		for i, w := range waiters {
			if w == ch {
				blpop.Store.Waiters[key] =
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
	if len(input)==0{
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

func (llen LLenCommand) Execute(args []string) string{
	key:=args[1]
	temp, ok :=llen.Store.Lists[key]

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
	for i := len(temp.listleft)-1; i >= 0; i-- {
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
	fields := make(map[string]string)

	for i := 3; i < len(args); i += 2 {
		fields[args[i]] = args[i+1]
	}

	entry := StreamEntry{
		ID:     finalID,
		Fields: fields,
	}

	stream.Entries = append(stream.Entries, entry)

	stream.TopId_time = ms
	stream.TopId_seq = seq

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


