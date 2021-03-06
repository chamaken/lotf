package main

import (
	"container/list"
	"encoding/json"
	"fmt"
	"github.com/chamaken/lotf"
	"github.com/golang/glog"
	"html/template"
	"net"
	"net/http"
	"strings"
	"time"
)

type TemplateRC struct {
	Title    string
	JsonPath string
	Expire   int
}

type JsonRC struct {
	Lines []string
	Error string
}

const (
	NEXT_SUFFIX = "/nextlines"
	COOKIE_NAME = "lotf"
)

var cfg *config
var cookies *TickMap
var templates = make(map[string]*template.Template)
var tails = make(map[string]lotf.Tail)

func makeJsonRC(t lotf.Tail) *JsonRC {
	l := list.New()
	for {
		if s := t.Next(); s == nil {
			break
		} else {
			l.PushBack(s)
		}
	}

	lines := make([]string, l.Len())
	i := 0
	for e := l.Front(); e != nil; e = e.Next() {
		lines[i] = *(e.Value.(*string))
		i++
	}
	m := &JsonRC{Lines: lines, Error: ""}
	return m
}

func handleNext(w http.ResponseWriter, r *http.Request, tail lotf.Tail) {
	w.Header().Set("Content-Type", "application/json")
	cookie, err := r.Cookie(COOKIE_NAME)
	if err != nil {
		w.Write([]byte(fmt.Sprintf("{\"error\": \"require cookie enable\"}")))
		return
	}
	v, err := cookies.Get(cookie.Value)
	if err != nil {
		w.Write([]byte(fmt.Sprintf("{\"error\": \"%s\"}", err)))
		return
	}
	if v == nil {
		// w.Write([]byte(fmt.Sprintf("{\"error\": \"%s\"}", err)))
		http.Redirect(w, r, strings.TrimSuffix(r.URL.Path, NEXT_SUFFIX), http.StatusFound)
		return
	}

	js, err := json.Marshal(makeJsonRC(v.(lotf.Tail)))
	if err != nil {
		w.Write([]byte(fmt.Sprintf("{\"error\": \"%s\"}", err)))
		return
	}
	w.Write(js)
}

func handleFirst(w http.ResponseWriter, r *http.Request, tail lotf.Tail, name string) {
	uuid, err := cookies.Add(tail.Clone())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	http.SetCookie(w, &http.Cookie{
		Name:  COOKIE_NAME,
		Value: uuid,
		Path:  cfg.root,
	})
	rc := &TemplateRC{
		Title:    fmt.Sprintf("%s", tail),
		JsonPath: r.URL.Path + NEXT_SUFFIX,
		Expire:   cfg.interval * 1000 / 2,
	}
	if err := templates[name].ExecuteTemplate(w, cfg.lotfs[name].template, rc); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	var tail lotf.Tail
	var found bool
	rpath := r.URL.Path[len(cfg.root):]
	if strings.HasSuffix(rpath, NEXT_SUFFIX) {
		key := rpath[:len(rpath)-len(NEXT_SUFFIX)]
		if tail, found = tails[key]; !found {
			http.NotFound(w, r)
			return
		}
		handleNext(w, r, tail)
	} else {
		if tail, found = tails[rpath]; !found {
			http.NotFound(w, r)
			return
		}
		handleFirst(w, r, tail, rpath)
	}
}

func main() {
	var err error

	cfg, err = parseFlags()
	if err != nil {
		glog.Fatalf("config error: %s", err)
	}

	cookies = NewTickMap(time.Duration(cfg.interval) * time.Second)
	watcher, err := lotf.NewTailWatcher()
	if err != nil {
		glog.Fatalf("NewTailWatcher: %s", err)
	}
	go func() {
		for err := range watcher.Error {
			glog.Errorf("error from watcher: %s", err)
		}
	}()

	templateNames := make(map[string]*template.Template)
	defaultTemplate := template.Must(template.ParseFiles(cfg.template))
	templates[defaultTemplate.Name()] = defaultTemplate
	for k, v := range cfg.lotfs {
		glog.Infof("creating tail: %s", v.filename)
		t, err := watcher.Add(v.filename, cfg.buflines, v.filter, cfg.lastlines)
		if err != nil {
			glog.Fatalf("Add to watcher - %s: %s", v.filename, err)
		}
		if len(v.template) == 0 {
			templates[k] = defaultTemplate
		} else {
			t := template.Must(template.ParseFiles(v.template))
			if _, found := templateNames[t.Name()]; !found {
				templateNames[t.Name()] = t
			}
			templates[k] = templateNames[t.Name()]
		}
		tails[k] = t
	}

	http.HandleFunc(cfg.root, handler)
	l, err := net.Listen("tcp", cfg.addr)
	if err != nil {
		glog.Fatalf("listen: %s", err)
	}
	s := &http.Server{}
	glog.Infof("start serving - addr: %s, path: %s", cfg.addr, cfg.root)
	if err := s.Serve(l); err != nil {
		glog.Fatalf("http.Serve: %s", err)
	}
}
