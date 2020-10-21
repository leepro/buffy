package proxy

import (
	"errors"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
)

const (
	TypeRespond = "respond"
	TypeProxy   = "proxy"
)

type EndpointHandler struct {
	def      *EndpointDef
	upstream *Upstream

	maxConn int
	curConn int
	counter uint32

	sync.Mutex
}

func NewEndpoint(e EndpointDef) (*Endpoint, error) {
	ep := &Endpoint{
		Def: &e,
		Handler: &EndpointHandler{
			def:      &e,
			maxConn:  e.MaxQueue,
			curConn:  0,
			upstream: nil,
		},
	}
	return ep, nil
}

func (eh *EndpointHandler) RegisterRoute(mux *http.ServeMux, upstream *Upstream) error {
	epf := eh.def
	var _proxy *httputil.ReverseProxy

	switch epf.Type {
	case TypeProxy:
		if upstream == nil {
			return errors.New("must provide 'upstream'")
		}

		upURL, err := url.Parse(upstream.Def.Endpoint)
		if err != nil {
			return err
		}

		_proxy = httputil.NewSingleHostReverseProxy(upURL)
		_proxy.Transport = &MyTransport{
			timeout: epf.Timeout,
		}
	}

	_handle := func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[endpoint(%d):%s:'%s'] %s\n", atomic.AddUint32(&eh.counter, 1), epf.Id, epf.Desc, r.URL)

		switch epf.Type {
		case TypeProxy:
			if eh.IsReachedMaxQueue() {
				content, err := epf.GetResponseWithName(NameHitMaxQueue)
				if err != nil {
					w.WriteHeader(http.StatusNotFound)
					w.Write([]byte("not found a response body for code 'hit_max_queue'"))
					return
				}

				w.WriteHeader(http.StatusOK)
				w.Write([]byte(content))
				return
			} else {
				// forward the request with the replacement of hostname
				eh.In()
				{
					r.Host = r.URL.Host
					_proxy.ServeHTTP(w, r)
				}
				eh.Out()
			}

		case TypeRespond:
			content, err := epf.GetResponseWithName(NameOK)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte("not found a response body for code 200"))
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

	eh.curConn++
}

func (eh *EndpointHandler) Out() {
	eh.Lock()
	defer eh.Unlock()

	eh.curConn--
}

func (eh *EndpointHandler) IsReachedMaxQueue() bool {
	eh.Lock()
	defer eh.Unlock()

	if eh.curConn < eh.maxConn {
		return false
	}

	return true
}
