package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/chamaken/lotf"
	"github.com/coreos/go-log/log"
	"io"
	"os"
	"strings"
)

var logfileFlag string
var loglevelFlag string
var rcfileFlag string
var pidfileFlag string

func init() {
	flag.StringVar(&logfileFlag, "o", "", "logfile or os.Stderr")
	flag.StringVar(&loglevelFlag, "l", "notice", "loglevel string, default notice")
	flag.StringVar(&rcfileFlag, "c", "config.json", "config filename")
	flag.StringVar(&pidfileFlag, "p", "", "pid filename")
}

type Config struct {
	Address  string
	Root     string
	Template string
	Duration int
	Lotfs    []LotfConfig
}

type LotfConfig struct {
	Name      string
	File      string
	Filter    string
	Buflines  int
	Lastlines int
}

type config struct {
	addr     string
	root     string
	template string
	duration int
	lotfs    map[string]*lotfConfig
}

type lotfConfig struct {
	filename  string
	filter    lotf.Filter
	buflines  int
	lastlines int
}

func makeResources(fname string) (*config, error) {
	r, err := os.Open(fname)
	if err != nil {
		return nil, err
	}

	s := &Config{Lotfs: make([]LotfConfig, 0)}
	dec := json.NewDecoder(r)
	for {
		if err := dec.Decode(s); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
	}

	lotfs := make(map[string]*lotfConfig)
	for _, v := range s.Lotfs {
		if _, found := lotfs[v.Name]; found {
			logger.Fatalf("founnd dup name: %s", v.Name)
		}
		var filter lotf.Filter
		if len(v.Filter) > 0 {
			filter, err = lotf.RegexpFilter(v.Filter)
			if err != nil {
				logger.Fatalf("create filter: %s", v.Filter)
			}
		} else {
			filter = nil
		}
		lotfs[v.Name] = &lotfConfig{
			filename:  v.File,
			filter:    filter,
			buflines:  v.Buflines,
			lastlines: v.Lastlines,
		}
	}

	if s.Root[len(s.Root)-1] != '/' {
		s.Root += "/"
	}
	return &config{
		addr:     s.Address,
		root:     s.Root,
		template: s.Template,
		duration: s.Duration,
		lotfs:    lotfs}, nil
}

func parseFlags() (*config, error) {
	flag.Parse()
	if flag.NArg() > 0 {
		return nil, errors.New(fmt.Sprintf("invalid arg(s): %s", flag.Args()))
	}

	level := log.PriNotice
	for k := log.PriEmerg; k <= log.PriDebug; k++ {
		v := strings.ToLower(fmt.Sprintf("%s", k))[:3]
		if strings.ToLower(loglevelFlag)[:3] == v {
			level = k
			break
		}
	}

	f := os.Stderr
	if len(logfileFlag) > 0 {
		var err error
		f, err = os.OpenFile(logfileFlag, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640)
		if err != nil {
			return nil, err
		}
	}
	logger = log.New("", true,
		log.PriorityFilter(
			level,
			log.WriterSink(f, log.RichFormat, log.RichFields),
		))

	resources, err := makeResources(rcfileFlag)
	if err != nil {
		return nil, err
	}

	if len(pidfileFlag) > 0 {
		pidfile, err := os.Create(pidfileFlag)
		if err != nil {
			return nil, err
		}
		defer pidfile.Close()
		fmt.Fprintf(pidfile, "%d\n", os.Getpid())
	}

	return resources, nil
}
