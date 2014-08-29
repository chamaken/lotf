package main

import (
	"time"
	"testing"
)

func TestNew(t *testing.T) {
	tm := NewTickMap(60 * time.Second)
	if tm.IsDone() {
		t.Fatalf("not running")
	}
	if tm.Len() != 0 {
		t.Fatalf("expect empty but lentgh: %d", tm.Len())
	}
	tm.Destroy()
	if !tm.IsDone() {
		t.Fatalf("still running")
	}
}

func TestSimple(t *testing.T) {
	tm := NewTickMap(60 * time.Second)
	defer tm.Destroy()

	k, err := tm.Add(1)
	if err != nil {
		t.Fatalf("Add - got error: %v", err)
	}

	v, err := tm.Get(k)
	if err != nil {
		t.Fatalf("Get - got error: %v", err)
	}

	switch v.(type) {
	case int:
	default:
		t.Fatalf("expect int but got: %T", v)
	}
	if v.(int) != 1 {
		t.Fatalf("expect 1 but got %d", v.(int))
	}

	if tm.Len() != 1 {
		t.Fatalf("expect length 1, but got: %d", tm.Len())
	}

	v, err = tm.Get(k)
	if err != nil {
		t.Fatalf("Get - got error: %v", err)
	}
}

func TestExpired(t *testing.T) {
	tm := NewTickMap(2 * time.Second)
	defer tm.Destroy()

	k, err := tm.Add("teststring")
	if err != nil {
		t.Fatalf("Add - got error: %v", err)
	}
	v, err := tm.Get(k)
	if err != nil {
		t.Fatalf("Get - got error: %v", err)
	}
	if v == nil {
		t.Fatalf("expect non-nil value, but got nil")
	}

	time.Sleep(4 * time.Second)
	v, err = tm.Get(k)
	if err != nil {
		t.Fatalf("Get - got error: %v", err)
	}
	if v != nil {
		t.Fatalf("expect nil value, but got: %v", v)
	}
	if tm.Len() != 0 {
		t.Fatalf("expect empty, but length: %d", tm.Len())
	}
}
