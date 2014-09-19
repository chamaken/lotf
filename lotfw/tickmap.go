package main

import (
	"container/list"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"time"
)

// http://play.golang.org/p/4FkNSiUDMg
// newUUID generates a random UUID according to RFC 4122
func newUUID() (string, error) {
	uuid := make([]byte, 16)
	n, err := io.ReadFull(rand.Reader, uuid)
	if n != len(uuid) || err != nil {
		return "", err
	}
	// variant bits; see section 4.1.1
	uuid[8] = uuid[8]&^0xc0 | 0x80
	// version 4 (pseudo-random); see section 4.1.3
	uuid[6] = uuid[6]&^0xf0 | 0x40
	return fmt.Sprintf("%x-%x-%x-%x-%x", uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:]), nil
}

type opcode int

const (
	GET opcode = iota
	ADD
	DONE
)

type command struct {
	code  opcode
	param interface{}
}

type retval struct {
	val interface{}
	err error
}

type TickMap struct {
	duration time.Duration
	fifo     *list.List
	vals     map[string]*list.Element
	cmd      chan *command
	rc       chan *retval
	done     bool
}

// value for TickMap.fifo and TickMap.vals
type element struct {
	key    string
	expire int64
	val    interface{}
}

func NewTickMap(d time.Duration) *TickMap {
	tm := &TickMap{
		fifo:     list.New(),
		cmd:      make(chan *command),
		duration: d,
		vals:     make(map[string]*list.Element),
		rc:       make(chan *retval),
		done:     false,
	}
	go tm.run()

	return tm
}

func (tm *TickMap) expire() {
	now := time.Now().Unix()
	for e := tm.fifo.Front(); e != nil; e = e.Next() {
		v := e.Value.(*element)
		if v.expire > now {
			break
		}
		delete(tm.vals, v.key)
		tm.fifo.Remove(e) // or create new list and add e?
	}
}

func (tm *TickMap) run() {
	tick := time.NewTicker(tm.duration)
	defer func() { tm.done = true }()
	for {
		select {
		case cmd := <-tm.cmd:
			if cmd.code == DONE {
				close(tm.cmd)
				close(tm.rc)
				return
			}
			rc, err := tm.handle(cmd)
			tm.rc <- &retval{rc, err}
		case <-tick.C:
			tm.expire()
		}
	}
}

func (tm *TickMap) handle(cmd *command) (interface{}, error) {
	switch cmd.code {
	case ADD: // cmd.val is just interface{} value
		var uuid string
		var err error
		for {
			uuid, err = newUUID()
			if err != nil {
				return nil, err
			}
			if _, found := tm.vals[uuid]; !found {
				break
			}
		}

		e := &element{uuid, time.Now().Unix(), cmd.param}
		tm.vals[uuid] = tm.fifo.PushBack(e)
		return uuid, nil

	case GET: // cmd.val is UUID
		v, found := tm.vals[cmd.param.(string)]
		if !found {
			return nil, nil
		}
		e := v.Value.(*element)
		e.expire = time.Now().Add(tm.duration).Unix()
		tm.fifo.MoveToBack(v)
		return e.val, nil
	default:
		return nil, errors.New(fmt.Sprintf("invalid opcode: %v", cmd.code))
	}
}

func (tm *TickMap) Add(val interface{}) (string, error) {
	tm.cmd <- &command{ADD, val}
	rc := <-tm.rc
	return rc.val.(string), rc.err
}

func (tm *TickMap) Get(uuid string) (interface{}, error) {
	tm.cmd <- &command{GET, uuid}
	rc := <-tm.rc
	return rc.val, rc.err
}

func (tm *TickMap) Destroy() {
	if tm.done {
		return
	}
	tm.cmd <- &command{DONE, nil}
	// wait closing channel
	<-tm.rc
	return

}

func (tm *TickMap) Len() int {
	if tm.fifo.Len() != len(tm.vals) {
		panic(fmt.Sprintf("invalid internal length - list: %d, map: %d",
			tm.fifo.Len(), len(tm.vals)))
	}
	return len(tm.vals)
}

func (tm *TickMap) IsDone() bool {
	return tm.done
}
