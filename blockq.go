// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is gdonened by a BSD-style
// license that can be found in the LICENSE file.
//
// このコードは Go package の container/list パッケージを参考にしたのでライセンス
// はこれに準じます。
//
// The license is same as golang since these code refers to container/list
// package in Go
// 
// Blockq は単純で稚拙な一方向の同期リストです。初期生成時にサイズを指定して Add
// で末尾に要素を加える時、この初期指定サイズを超えると先頭の要素を削除します。ま
// た AddHead で先頭に要素を追加するにあたっては初期サイズを超えるとエラーを返し
// ます。WaitHead() で先頭要素が加わるまでブロック、また Element の WaitNext()で
// 末尾に新たな要素が加わるまでブロックします。要素を加えずにブロックを解除するに
// は Done() を呼び出します。
//
// Blockq is a simple and silly single list implemetation. The size needs
// spcified on initialization and it pop first element if Add() and size exceeds
// it. And AddHead() returns err if it will exceeds size. WaitHead() will block
// until the first element, head is added. WaitNext() Element receiver will
// block until the next of this element is added. Done() needs to call to
// release these block.

package lotf

import (
	"sync"
	"fmt"
)

// Element is an element in the linked list.
type Element struct {
	next *Element
	list *Blockq
	Value interface{}
}

// Blockq represents a push only list.
type Blockq struct {
	head, tail *Element
	len	int
	limit	int
	done	bool
	lock	*sync.RWMutex
	cond	*sync.Cond
}


// New returns an initialized list.
func NewBlockq(size int) (*Blockq, error) {
	if size <= 0 {
		return nil, fmt.Errorf("invalid size: %d", size)
	}

	l := new(Blockq)
	e := &Element{nil, l, (interface{})(nil)}
	l.head = e
	l.tail = e

	l.len = 0
	l.limit = size
	l.done = false
	l.lock = new(sync.RWMutex)
	l.cond = sync.NewCond(l.lock)

	return l, nil
}


func (l *Blockq) Head() *Element { return l.head.next }
func (l *Blockq) Tail() *Element {
	l.lock.RLock()
	defer l.lock.RUnlock()
	if l.head == l.tail {
		return nil
	} else {
		return l.tail
	}
}
func (e *Element) Next() *Element { return e.next }


// blocking
func (l *Blockq) WaitHead() *Element {
	return l.head.WaitNext()
}


func (e *Element) WaitNext() *Element {
	e.list.lock.Lock()
	defer e.list.lock.Unlock()
	// defer func() { e.list.done = false }()

	w := e.next
	for w == nil && ! e.list.done {
		e.list.cond.Wait()
		w = e.next
	}

	return w
}


// add the value at the tail and returns head Element if the limit exceeds.
func (l *Blockq) Add(value interface{}) *Element {
	e := &Element{nil, l, value}

	l.lock.Lock()
	defer l.cond.Broadcast()
	defer l.lock.Unlock()

	l.tail.next = e
	l.tail = e
	l.len++
	if l.len > l.limit {
		e := l.head.next
		l.head.next = e.next
		l.len--
		return e
	}

	return nil
}

func (l *Blockq) AddHead(value interface{}) error {
	if l.len >= l.limit {
		return fmt.Errorf("this queue is full: %d", l.len)
	}

	e := &Element{l.head.next, l, value}

	l.lock.Lock()
	defer l.cond.Broadcast()
	defer l.lock.Unlock()

	l.head.next = e
	if l.len == 0 {
		l.tail = e
	}
	l.len++
	return nil
}


func (l *Blockq) Done() {
	l.lock.Lock() // barrier?
	defer l.cond.Broadcast()
	defer l.lock.Unlock()

	l.done = true
}
