package main

import (
	"fmt"
	"net"
	"github.com/chamaken/lotf"
	"github.com/chamaken/logger"
)


type StreamServer struct {
	tail lotf.Tail
	listener *net.TCPListener
	done chan bool
}


func NewTCPServer(t lotf.Tail, addr *net.TCPAddr)(*StreamServer, error) {
	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return nil, err
	}
	return &StreamServer{t, listener, make(chan bool, 1)}, nil
}


func serve(conn net.Conn, t lotf.Tail, errch chan<- error) {
	defer conn.Close()
	for s := t.Next(); s != nil; s = t.Next() {
		b := []byte(fmt.Sprintf("%s\n", *s))
		if n, err := conn.Write(b); err != nil {
			logger.Error("write error to [%s]: %s", conn.RemoteAddr(), err)
			break
		} else if n != len(b) {
			logger.Warning("could not write at once, writing: %d, written: %d", len(b), n)
		}
	}
}


func (svr *StreamServer) Run(errch chan<- error) {
	done := false
	for !done {
		conn, err := svr.listener.Accept()
		select { case done = <- svr.done: default: }
		if done { break }
		if err != nil {
			logger.Error("listener accept: %s", err)
			errch <- err
		} else {
			go serve(conn, svr.tail.Clone(), errch)
		}
	}
	logger.Info("exit Run gracefully")
}


func (svr *StreamServer) Done() error {
	svr.done <- true
	if err := svr.listener.Close(); err != nil {
		logger.Warning("failed to close: %s", err)
		return err
	}
	return nil
}
