package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/chamaken/lotf"
	"io"
	"os"
)

var rcfileFlag string
var pidfileFlag string

func init() {
	flag.StringVar(&rcfileFlag, "c", "config.json", "config filename")
	flag.StringVar(&pidfileFlag, "p", "", "pid filename")
}

type Config struct {
	Address   string
	Root      string
	Template  string
	Interval  int
	Buflines  int
	Lastlines int
	Lotfs     []LotfConfig
}

type LotfConfig struct {
	Name     string
	File     string
	Filter   string
	Template string
}

type config struct {
	addr      string
	root      string
	template  string
	interval  int
	buflines  int
	lastlines int
	lotfs     map[string]*lotfConfig
}

type lotfConfig struct {
	filename string
	filter   lotf.Filter
	template string
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
		// XXX: check required json entries
		if len(v.Name) == 0 {
			return nil, errors.New("no name specified in lotfs")
		}
		if _, found := lotfs[v.Name]; found {
			return nil, errors.New(fmt.Sprintf("founnd dup name: %s", v.Name))
		}
		var filter lotf.Filter
		if len(v.Filter) > 0 {
			filter, err = lotf.RegexpFilter(v.Filter)
			if err != nil {
				return nil, errors.New(fmt.Sprintf("create filter: %s", v.Filter))
			}
		} else {
			filter = nil
		}
		if len(v.File) == 0 {
			return nil, errors.New(fmt.Sprintf("no file specified: %s", v.Name))
		}

		lotfs[v.Name] = &lotfConfig{
			filename: v.File,
			filter:   filter,
			template: v.Template,
		}
	}

	if len(s.Address) == 0 {
		return nil, errors.New("address is not specified")
	}
	if len(s.Root) == 0 {
		return nil, errors.New("root is not specified")
	}
	if len(s.Template) == 0 {
		return nil, errors.New("default template is not specified")
	}
	if s.Interval == 0 {
		return nil, errors.New("interval is not specified")
	}
	if s.Buflines == 0 {
		return nil, errors.New("buflines is not specified")
	}
	if s.Lastlines == 0 {
		return nil, errors.New("lastlines is not specified")
	}
	if len(lotfs) == 0 {
		return nil, errors.New("no lotf specified")
	}

	if s.Root[len(s.Root)-1] != '/' {
		s.Root += "/"
	}
	return &config{
		addr:      s.Address,
		root:      s.Root,
		template:  s.Template,
		interval:  s.Interval,
		buflines:  s.Buflines,
		lastlines: s.Lastlines,
		lotfs:     lotfs,
	}, nil
}

func parseFlags() (*config, error) {
	flag.Parse()
	if flag.NArg() > 0 {
		return nil, errors.New(fmt.Sprintf("invalid arg(s): %s", flag.Args()))
	}

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
