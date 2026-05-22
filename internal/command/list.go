package command

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/codecrafters-io/redis-starter-go/internal/store"
)

type RpushCommand struct{ Store *store.DataStore }

func (rpush RpushCommand) Execute(args []string) string {
	key := args[1]
	rpush.Store.Lock()

	waiters := rpush.Store.ListWaiters[key]

	if len(waiters) > 0 {
		ch := waiters[0]
		rpush.Store.ListWaiters[key] = waiters[1:]

		val := args[2]

		rpush.Store.Unlock()

		ch <- []string{key, val}

		return fmt.Sprintf(":%d\r\n", 1)
	}

	temp, ok := rpush.Store.Lists[key]
	if !ok {
		temp = store.Variables{}
	}

	for i := 2; i < len(args); i++ {
		temp.ListRight = append(temp.ListRight, args[i])
	}
	rpush.Store.Lists[key] = temp
	rpush.Store.KeyVersion[key]++
	total := len(temp.ListLeft) + len(temp.ListRight)

	rpush.Store.Unlock()

	return fmt.Sprintf(":%d\r\n", total)
}

type LPushCommand struct{ Store *store.DataStore }

func (lpush LPushCommand) Execute(args []string) string {
	key := args[1]
	lpush.Store.Lock()

	waiters := lpush.Store.ListWaiters[key]

	if len(waiters) > 0 {
		ch := waiters[0]
		lpush.Store.ListWaiters[key] = waiters[1:]

		val := args[2]

		lpush.Store.Unlock()

		ch <- []string{key, val}

		return fmt.Sprintf(":%d\r\n", 1)
	}

	temp, ok := lpush.Store.Lists[key]
	if !ok {
		temp = store.Variables{}
	}

	for i := 2; i < len(args); i++ {
		temp.ListLeft = append(temp.ListLeft, args[i])
	}

	lpush.Store.Lists[key] = temp
	lpush.Store.KeyVersion[key]++
	total := len(temp.ListLeft) + len(temp.ListRight)

	lpush.Store.Unlock()
	return fmt.Sprintf(":%d\r\n", total)
}

type LPopCommand struct{ Store *store.DataStore }

func (lpop LPopCommand) Execute(args []string) string {
	key := args[1]

	lpop.Store.Lock()
	defer lpop.Store.Unlock()

	lft := 1
	if len(args) > 2 {
		lft, _ = strconv.Atoi(args[2])
	}

	temp, ok := lpop.Store.Lists[key]
	if !ok {
		return "$-1\r\n"
	}

	popkey := make([]string, 0, lft)
	for i := 0; i < lft; i++ {
		if len(temp.ListLeft) != 0 {
			popkey = append(popkey, temp.ListLeft[len(temp.ListLeft)-1])
			temp.ListLeft = temp.ListLeft[0 : len(temp.ListLeft)-1]
		} else if len(temp.ListRight) != 0 {
			popkey = append(popkey, temp.ListRight[0])
			temp.ListRight = temp.ListRight[1:]
		} else if len(popkey) == 0 {
			return "$-1\r\n"
		} else {
			break
		}
	}

	if len(temp.ListLeft) == 0 && len(temp.ListRight) == 0 {
		delete(lpop.Store.Lists, key)
	} else {
		lpop.Store.Lists[key] = temp
	}

	lpop.Store.KeyVersion[key]++

	var builder strings.Builder
	if len(popkey) == 1 {
		return fmt.Sprintf("$%d\r\n%s\r\n", len(popkey[0]), popkey[0])
	}
	builder.WriteString(fmt.Sprintf("*%d\r\n", len(popkey)))
	for i := 0; i < len(popkey); i++ {
		val := popkey[i]
		builder.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(val), val))
	}
	return builder.String()
}

type BLPopCommand struct{ Store *store.DataStore }

func (blpop BLPopCommand) Execute(args []string) string {
	key := args[1]

	timeout := 0.0
	if len(args) > 2 {
		timeout, _ = strconv.ParseFloat(args[2], 64)
	}

	blpop.Store.Lock()

	temp, ok := blpop.Store.Lists[key]

	// immediate pop
	if ok && (len(temp.ListLeft) != 0 || len(temp.ListRight) != 0) {
		var val string

		if len(temp.ListLeft) != 0 {
			val = temp.ListLeft[len(temp.ListLeft)-1]
			temp.ListLeft = temp.ListLeft[:len(temp.ListLeft)-1]
		} else {
			val = temp.ListRight[0]
			temp.ListRight = temp.ListRight[1:]
		}

		blpop.Store.Lists[key] = temp
		blpop.Store.Unlock()

		return encodeArray([]string{key, val})
	}

	ch := make(chan []string, 1)
	blpop.Store.ListWaiters[key] = append(blpop.Store.ListWaiters[key], ch)

	blpop.Store.Unlock()

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
		blpop.Store.Lock()

		waiters := blpop.Store.ListWaiters[key]
		for i, w := range waiters {
			if w == ch {
				blpop.Store.ListWaiters[key] = append(waiters[:i], waiters[i+1:]...)
				break
			}
		}

		blpop.Store.Unlock()

		return "*-1\r\n"
	}
}

type LLenCommand struct{ Store *store.DataStore }

func (llen LLenCommand) Execute(args []string) string {
	key := args[1]
	temp, ok := llen.Store.Lists[key]
	if !ok {
		temp = store.Variables{}
	}
	total := len(temp.ListLeft) + len(temp.ListRight)
	return fmt.Sprintf(":%d\r\n", total)
}

type LRangeCommand struct{ Store *store.DataStore }

func (lrange LRangeCommand) Execute(args []string) string {
	key := args[1]
	lft, _ := strconv.Atoi(args[2])
	rgt, _ := strconv.Atoi(args[3])

	temp, ok := lrange.Store.Lists[key]
	if !ok {
		return "*0\r\n"
	}

	combined := make([]string, 0)
	for i := len(temp.ListLeft) - 1; i >= 0; i-- {
		combined = append(combined, temp.ListLeft[i])
	}
	combined = append(combined, temp.ListRight...)

	size := len(combined)
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

	for i := start; i <= end; i++ {
		val := combined[i]
		builder.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(val), val))
	}
	return builder.String()
}

// encodeArray encodes a slice of strings as a RESP array (or a null array when
// empty). Shared by the list commands.
func encodeArray(input []string) string {
	var builder strings.Builder
	if len(input) == 0 {
		return "*-1\r\n"
	}

	builder.WriteString(fmt.Sprintf("*%d\r\n", len(input)))
	for i := 0; i < len(input); i++ {
		val := input[i]
		builder.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(val), val))
	}
	return builder.String()
}
