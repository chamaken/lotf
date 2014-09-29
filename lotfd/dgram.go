package main

import (
	"fmt"
	"github.com/chamaken/lotf"
	"github.com/golang/glog"
	"io"
	"net"
)

type DgramServer struct {
	tail lotf.Tail
	conn io.WriteCloser
}

func NewUDPServer(t lotf.Tail, raddr *net.UDPAddr) (*DgramServer, error) {
	conn, err := net.DialUDP("udp4", nil, raddr)
	if err != nil {
		return nil, err
	}
	return &DgramServer{t, io.WriteCloser(conn)}, nil
}

func NewUnixgramServer(t lotf.Tail, raddr *net.UnixAddr) (*DgramServer, error) {
	conn, err := net.DialUnix("unixgram", nil, raddr)
	if err != nil {
		return nil, err
	}
	return &DgramServer{t, io.WriteCloser(conn)}, nil
}

// loop will stop by Tail.Done()
func (svr *DgramServer) Run(errch chan<- error) {
	for s := svr.tail.WaitNext(); s != nil; s = svr.tail.WaitNext() {
		b := []byte(fmt.Sprintf("%s\n", *s))
		if n, err := svr.conn.Write(b); err != nil {
			glog.Errorf("connection write: %s", err)
			errch <- err
		} else if n != len(b) {
			glog.Infof("could not write at once, writing: %d, written: %d", len(b), n)
		}
	}
	glog.Info("exit Run gracefully")
}

func (svr *DgramServer) Done() error {
	EOM := []byte{}

	if _, err := svr.conn.Write(EOM); err != nil {
		glog.Infof("connection final write: %s", err)
		return err
	}

	if err := svr.conn.Close(); err != nil {
		glog.Infof("connection close: %s", err)
		return err
	}
	return nil
}
