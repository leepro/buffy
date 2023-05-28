package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	TypeRespond = "respond"
	TypeProxy   = "proxy"
)

type EndpointDef struct {
	Id        string                `json:"id"         yaml:"id"`
	Desc      string                `json:"desc"       yaml:"desc"`
	Path      string                `json:"path"       yaml:"path"`
	Type      string                `json:"type"       yaml:"type"`
	Upstream  []string              `json:"upstream"   yaml:"upstream"`
	ProxyMode string                `json:"proxy_mode" yaml:"proxy_mode"`
	Timeout   int                   `json:"timeout"    yaml:"timeout"`
	MaxQueue  int                   `json:"max_queue"  yaml:"max_queue"`
	Methods   []string              `json:"methods"    yaml:"methods"`
	Response  []EndpointResponseDef `json:"response"   yaml:"response"`
}

type EndpointResponseDef struct {
	Name       string `json:"name"        yaml:"name"`
	ReturnCode int    `json:"return_code" yaml:"return_code"`
	Content    string `json:"content"     yaml:"content"`
}

type EndpointHandler struct {
	ctx      context.Context
	def      *EndpointDef
	upstream *Upstream
	notiC    chan string
	handler  http.HandlerFunc

	MaxConn int                   `json:"maxconn"`
	CurConn int                   `json:"curconn"`
	Counter uint32                `json:"counter"`
	Conns   map[string]*ConnState `json:"conns"`

	sync.Mutex
}

type ConnState struct {
	RemoteAddr string `json:"remote_addr"`
	CreatedAt  int64  `json:"created_at"`
}

func NewEndpoint(ctx context.Context, e EndpointDef, notiC chan string) (*Endpoint, error) {
	ep := &Endpoint{
		Id:   e.Id,
		Path: e.Path,
		Def:  &e,
		Handler: &EndpointHandler{
			ctx:      ctx,
			notiC:    notiC,
			def:      &e,
			MaxConn:  e.MaxQueue,
			CurConn:  0,
			upstream: nil,
			Conns:    make(map[string]*ConnState),
		},
	}
	return ep, nil
}

func (eh *EndpointHandler) RegisterRoute(mux *http.ServeMux, upstream *Upstream) error {
	epf := eh.def
	cfg := eh.ctx.Value(ctxKeyConfig).(*BuffyConfig)

	var _handle http.HandlerFunc

	switch epf.Type {
	case TypeProxy:
		if upstream == nil {
			return errors.New("must provide 'upstream'")
		}

		// attach the upstream
		eh.upstream = upstream
		if err := eh.upstream.CreateReverseProxy(epf.ProxyMode, epf.Timeout); err != nil {
			return err
		}

		_handle = func(w http.ResponseWriter, r *http.Request) {
			log.Printf("[endpoint(%d):%s:'%s'] %s\n", atomic.AddUint32(&eh.Counter, 1), epf.Id, epf.Desc, r.URL)

			var code int
			var content string
			var err error

			if eh.IsReachedMaxQueue() {
				code, content, err = epf.GetResponseWithName(NameHitMaxQueue, cfg.BasePath)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte("buffy[yaml]: not found a response body for code 'hit_max_queue' : " + err.Error()))
					return
				}
			} else {
				// forward the request with the replacement of hostname
				sid := eh.In(r)
				// IMPORTANT
				r.Host = r.URL.Host
				r.Header.Add("X-Buffy-URL", r.RequestURI)
				r.Header.Add("X-Buffy-Endpoint-ID", epf.Id)
				r.Header.Add("X-Buffy-Way", "up")
				eh.upstream.Forward(w, r)
				eh.Out(sid)
				return
			}

			// add default headers
			eh.addHeaders(w, r, epf)

			w.WriteHeader(code)
			w.Write([]byte(content))
		}

	case TypeRespond:
		_handle = func(w http.ResponseWriter, r *http.Request) {
			log.Printf("[endpoint(%d):%s:'%s'] %s\n", atomic.AddUint32(&eh.Counter, 1), epf.Id, epf.Desc, r.URL)

			var code int
			var content string
			var err error

			code, content, err = epf.GetResponseWithName(NameOK, cfg.BasePath)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("buffy[yaml]: not found a response body for code 200 : " + err.Error()))
				return
			}

			// process template
			content = eh.processTemplate(w, r, epf, content)

			// add default headers
			eh.addHeaders(w, r, epf)

			w.WriteHeader(code)
			w.Write([]byte(content))
		}
	}

	mux.HandleFunc(epf.Path, _handle)
	eh.handler = _handle

	return nil
}

func (eh *EndpointHandler) addHeaders(w http.ResponseWriter, r *http.Request, epf *EndpointDef) {
	w.Header().Add("X-Buffy-URL", r.RequestURI)
	w.Header().Add("X-Buffy-Endpoint-ID", epf.Id)
}

func (eh *EndpointHandler) processTemplate(w http.ResponseWriter, r *http.Request, epf *EndpointDef, content string) string {
	content = strings.ReplaceAll(content, "{{URL}}", r.RequestURI)
	content = strings.ReplaceAll(content, "{{ID}}", epf.Id)
	return content
}

func (eh *EndpointHandler) notify(msg string) {
	if len(eh.notiC) < cap(eh.notiC) {
		eh.notiC <- msg
	}
}

func (eh *EndpointHandler) In(r *http.Request) string {
	eh.Lock()
	defer eh.Unlock()

	ts := time.Now().Unix()
	sid := fmt.Sprintf("%x-%s", ts, r.RemoteAddr)
	eh.Conns[sid] = &ConnState{RemoteAddr: r.RemoteAddr, CreatedAt: ts}
	eh.CurConn++

	return sid
}

func (eh *EndpointHandler) MarshalJSON() ([]byte, error) {
	eh.Lock()
	defer eh.Unlock()

	return json.Marshal(struct {
		MaxConn int                   `json:"maxconn"`
		CurConn int                   `json:"curconn"`
		Counter uint32                `json:"counter"`
		Conns   map[string]*ConnState `json:"conns"`
	}{
		MaxConn: eh.MaxConn,
		CurConn: eh.CurConn,
		Counter: eh.Counter,
		Conns:   eh.Conns,
	})
}

func (cs *ConnState) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		RemoteAddr string `json:"remote_addr"`
		CreatedAt  int64  `json:"created_at"`
		Elasped    int64  `json:"elapsed"`
	}{
		RemoteAddr: cs.RemoteAddr,
		CreatedAt:  cs.CreatedAt,
		Elasped:    time.Now().Unix() - cs.CreatedAt,
	})
}

func (eh *EndpointHandler) Out(sid string) {
	eh.Lock()
	defer eh.Unlock()

	delete(eh.Conns, sid)
	eh.CurConn--
}

func (eh *EndpointHandler) IsReachedMaxQueue() bool {
	eh.Lock()
	defer eh.Unlock()

	if eh.CurConn < eh.MaxConn {
		return false
	}

	return true
}
