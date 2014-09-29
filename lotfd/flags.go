package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/chamaken/lotf"
	"io"
	"net"
	"os"
)

var rcfileFlag string
var pidfileFlag string
var lastlinesFlag int

func init() {
	flag.StringVar(&rcfileFlag, "c", "lotfd.json", "config filename")
	flag.StringVar(&pidfileFlag, "p", "", "pid filename")
	flag.IntVar(&lastlinesFlag, "n", 10, "last lines on startup")
}

type RCEntry struct {
	File     string
	Filter   string
	Udpaddr  string
	Tcpaddr  string
	Buflines int
}

type LTFResource struct {
	filename string
	filter   lotf.Filter
	tcpaddr  *net.TCPAddr
	udpaddr  *net.UDPAddr
	buflines int
}

func makeResources(fname string) ([]LTFResource, error) {
	var err error

	r, err := os.Open(fname)
	if err != nil {
		return nil, err
	}

	s := make([]RCEntry, 0)
	dec := json.NewDecoder(r)
	for {
		if err := dec.Decode(&s); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
	}

	t := make([]LTFResource, len(s))
	for i, e := range s {
		t[i].filename = e.File
		t[i].buflines = e.Buflines
		if len(e.Filter) > 0 {
			if t[i].filter, err = lotf.RegexpFilter(e.Filter); err != nil {
				return nil, err
			}
		}

		if len(e.Udpaddr) > 0 {
			if t[i].udpaddr, err = net.ResolveUDPAddr("udp4", e.Udpaddr); err != nil {
				return nil, err
			}
		}
		if len(e.Tcpaddr) > 0 {
			if t[i].tcpaddr, err = net.ResolveTCPAddr("tcp4", e.Tcpaddr); err != nil {
				return nil, err
			}
		}
	}

	return t, nil
}

func parseFlags() ([]LTFResource, int, error) {
	flag.Parse()
	if flag.NArg() > 0 {
		return nil, -1, errors.New(fmt.Sprintf("invalid arg(s): %s", flag.Args()))
	}

	resources, err := makeResources(rcfileFlag)
	if err != nil {
		return nil, -1, err
	}

	if len(pidfileFlag) > 0 {
		pidfile, err := os.Create(pidfileFlag)
		if err != nil {
			return nil, -1, err
		}
		defer pidfile.Close()
		fmt.Fprintf(pidfile, "%d\n", os.Getpid())
	}

	if lastlinesFlag < 0 {
		return nil, -1, errors.New("invalid last line")
	}

	return resources, lastlinesFlag, nil
}
