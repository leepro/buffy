package proxy

import (
	"context"
	"errors"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
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
	Response  []UpstreamResponseDef `json:"response"   yaml:"response"`
}

type EndpointHandler struct {
	ctx      context.Context
	def      *EndpointDef
	upstream *Upstream

	MaxConn int    `json:"maxConn"`
	CurConn int    `json:"curConn"`
	Counter uint32 `json:"counter"`

	sync.Mutex
}

func NewEndpoint(ctx context.Context, e EndpointDef) (*Endpoint, error) {
	ep := &Endpoint{
		Id:   e.Id,
		Path: e.Path,
		Def:  &e,
		Handler: &EndpointHandler{
			ctx:      ctx,
			def:      &e,
			MaxConn:  e.MaxQueue,
			CurConn:  0,
			upstream: nil,
		},
	}
	return ep, nil
}

func (eh *EndpointHandler) RegisterRoute(mux *http.ServeMux, upstream *Upstream) error {
	epf := eh.def
	var _handle http.HandlerFunc

	switch epf.Type {
	case TypeProxy:
		if upstream == nil {
			return errors.New("must provide 'upstream'")
		}

		// attach the upstream
		eh.upstream = upstream
		if err := eh.upstream.CreateReverseProxy(epf.Timeout); err != nil {
			return err
		}

		_handle = func(w http.ResponseWriter, r *http.Request) {
			log.Printf("[endpoint(%d):%s:'%s'] %s\n", atomic.AddUint32(&eh.Counter, 1), epf.Id, epf.Desc, r.URL)

			var content string
			var err error

			if eh.IsReachedMaxQueue() {
				content, err = epf.GetResponseWithName(NameHitMaxQueue)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte("buffy[yaml]: not found a response body for code 'hit_max_queue'"))
					return
				}
			} else {
				// forward the request with the replacement of hostname
				eh.In()
				r.Host = r.URL.Host
				eh.upstream.Forward(w, r)
				eh.Out()
				return
			}

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(content))
		}

	case TypeRespond:
		_handle = func(w http.ResponseWriter, r *http.Request) {
			log.Printf("[endpoint(%d):%s:'%s'] %s\n", atomic.AddUint32(&eh.Counter, 1), epf.Id, epf.Desc, r.URL)

			var content string
			var err error

			content, err = epf.GetResponseWithName(NameOK)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("buffy[yaml]: not found a response body for code 200"))
				return
			}

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(content))
		}
	}

	mux.HandleFunc(epf.Path, _handle)

	return nil
}

func (eh *EndpointHandler) In() {
	eh.Lock()
	defer eh.Unlock()

	eh.CurConn++
}

func (eh *EndpointHandler) Out() {
	eh.Lock()
	defer eh.Unlock()

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
