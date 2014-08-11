package lotf

import (
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	if _, err := NewBlockq(0); err != nil {
		if err == nil {
			t.Fatal("NewBlockq accept len 0")
		}
	}

	if _, err := NewBlockq(1); err != nil {
		if err == nil {
			t.Fatal("could not create blockq")
		}
	}
}

func TestNonBlock(t *testing.T) {
	q, _ := NewBlockq(4)
	if q.Head() != nil {
		t.Fatal("got a head value from empty queue")
	}
	if q.Tail() != nil {
		t.Fatal("got a tail value from empty queue")
	}

	q.Add(3)

	e := q.Head()
	if e == nil {
		t.Fatal("could not get head")
	}
	if e.Value.(int) != 3 {
		t.Fatal("got invalid value: %v", e.Value)
	}

	e = q.Tail()
	if e == nil {
		t.Fatal("could not get tail")
	}
	if e.Value.(int) != 3 {
		t.Fatalf("got invalid value: %v", e.Value)
	}

	if e.Next() != nil {
		t.Fatal("got element from which has not next")
	}
	q.Add(4)
	if e.Next().Value.(int) != 4 {
		t.Fatal("got invalid value by Next")
	}

	q.AddHead(2)
	q.AddHead(1)

	i := 1
	for e = q.Head(); e != nil; e = e.Next() {
		if e.Value.(int) != i {
			t.Fatalf("got invalid value by Next %d != %v", i, e.Value)
		}
		i++
	}

	if err := q.AddHead(0); err == nil {
		t.Fatalf("added limit exceeding to head")
	}

	e = q.Add(5)
	if e == nil || e.Value.(int) != 1 {
		t.Fatalf("got invalid overflowed element: %v", e.Value)
	}
}

func TestBlock(t *testing.T) {
	var e *Element
	ch := make(chan *Element)
	q, _ := NewBlockq(4)

	// WaitHead()
	for i := 0; i < 32; i++ {
		go func() {
			ch <- q.WaitHead()
		}()
	}
	time.Sleep(1 * time.Second)
	select {
	case e = <- ch:
	default:
	}
	if e != nil {
		t.Fatal("receive unreceivable element under normal")
	}
	q.Add(1)
	for i := 0; i < 32; i++ {
		e = <- ch
		if e.Value.(int) != 1 {
			t.Fatalf("receive invalid value: %v", e.Value)
		}
	}

	// WaitNext()
	var e2 *Element
	for i := 0; i < 32; i++ {
		go func() {
			ch <- e.WaitNext()
		}()
	}
	time.Sleep(1 * time.Second)
	select {
	case e2 = <- ch:
	default:
	}
	if e2 != nil {
		t.Fatalf("receive unreceivable element under normal: %v", e.Value)
	}
	q.Add(2)
	for i := 0; i < 32; i++ {
		e2 = <- ch
		if e2.Value.(int) != 2 {
			t.Fatalf("receive invalid value: %v", e2.Value)
		}
	}

	// Done()
	var e3 *Element
	for i := 0; i < 32; i++ {
		go func() {
			ch <- e2.WaitNext()
		}()
	}
	q.Done()

	for i := 0; i < 32; i++ {
		e3 = <- ch
		if e3 != nil {
			t.Fatalf("receive non-nil value: %v", e3.Value)
		}
	}
}
