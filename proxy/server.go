package proxy

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

const (
	NameHitMaxQueue = "hit_max_queue"
	NameHitTimeout  = "hit_timeout"
	NameOK          = "200"
)

type ProxyServer struct {
	Bind string
	Cfg  *BuffyConfig

	upstreams    []*Upstream
	endpoints    []*Endpoint
	tableIDtoMax map[string]int

	mux *http.ServeMux

	ctx context.Context
	sync.Mutex
}

type Upstream struct {
	Def     *UpstreamDef
	Handler *UpstreamHandler
}

type Endpoint struct {
	Def     *EndpointDef
	Handler *EndpointHandler
}

func ListenAndServe(cfg *BuffyConfig) (*ProxyServer, error) {
	ps := &ProxyServer{
		Cfg:          cfg,
		Bind:         cfg.ListenHostPort(),
		ctx:          context.Background(),
		tableIDtoMax: make(map[string]int),
		mux:          &http.ServeMux{},
	}

	if err := ps.Run(); err != nil {
		return nil, err
	}

	return ps, nil
}

func (ps *ProxyServer) Run() error {
	// create upstream pipelines
	if err := ps.CreateUpstreamHandlers(); err != nil {
		return err
	}

	// register endpoints
	if err := ps.RegisterEndpoints(); err != nil {
		return err
	}

	ps.mux.HandleFunc("/", ps.ProxyHandle)

	srv := &http.Server{
		Addr:    ps.Bind,
		Handler: ps.mux,
	}

	go func() {
		select {
		case <-ps.ctx.Done():
		}

		if err := srv.Shutdown(ps.ctx); err != nil {
			log.Printf("ProxyServer shutdown: err=%s\n", err)
			return
		}
	}()

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Printf("ProxyServer shutdown: err=%s\n", err)
		}
	}()

	return nil
}

func (ps *ProxyServer) ProxyHandle(w http.ResponseWriter, r *http.Request) {
	log.Printf("[ProxyHandle] url=[%s]\n", r.URL)
	ps.serveEndpoints(w, r)
}

func (ps *ProxyServer) serveEndpoints(w http.ResponseWriter, r *http.Request) {

}

func (ps *ProxyServer) CreateUpstreamHandlers() error {
	for _, u := range ps.Cfg.Upstreams {
		up, err := NewUpstream(u)
		if err != nil {
			return err
		}
		ps.upstreams = append(ps.upstreams, up)
	}

	return nil
}

func (ps *ProxyServer) LookupUpstreamWithIds(ids []string) (*UpstreamDef, error) {
	for _, u := range ps.upstreams {
		if u.Def.Id == ids[0] {
			return u.Def, nil
		}
	}
	return nil, errors.New("not found upstream with id: " + ids[0])
}

func (ps *ProxyServer) RegisterEndpoints() error {
	// go func() {
	// 	t := time.NewTicker(1 * time.Second)
	// 	for {
	// 		select {
	// 		case <-t.C:
	// 			log.Printf("[monitor] %#v\n", ps.tableIDtoMax)
	// 		}
	// 	}
	// }()

	for _, epdef := range ps.Cfg.Endpoints {
		endp, err := NewEndpoint(epdef)
		if err != nil {
			return err
		}
		ps.endpoints = append(ps.endpoints, endp)

		ep := epdef
		var _proxy *httputil.ReverseProxy
		switch ep.Type {
		case TypeProxy:
			upstreamURL, err := ps.LookupUpstreamWithIds(epdef.Upstream)
			if err != nil {
				return err
			}

			upURL, err := url.Parse(upstreamURL.Endpoint)
			if err != nil {
				return err
			}

			_proxy = httputil.NewSingleHostReverseProxy(upURL)
			_proxy.Transport = &MyTransport{}
		}

		ps.mux.HandleFunc(ep.Path, func(w http.ResponseWriter, r *http.Request) {
			log.Printf("[endpoint:%s:'%s'] %s\n", ep.Id, ep.Desc, r.URL)

			switch ep.Type {
			case TypeProxy:
				if ps.IsReachedMaxQueue(ep.Id, ep.MaxQueue) {
					content, err := ep.GetResponseWithName(NameHitMaxQueue)
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
					ps.In(ep.Id)
					r.Host = r.URL.Host
					_proxy.ServeHTTP(w, r)
					ps.Out(ep.Id)
				}

			case TypeRespond:
				content, err := ep.GetResponseWithName(NameOK)
				if err != nil {
					w.WriteHeader(http.StatusNotFound)
					w.Write([]byte("not found a response body for code 200"))
					return
				}

				w.WriteHeader(http.StatusOK)
				w.Write([]byte(content))
			}
		})
	}
	return nil
}

func (ps *ProxyServer) IsReachedMaxQueue(id string, max int) bool {
	ps.Lock()
	defer ps.Unlock()

	if n, ok := ps.tableIDtoMax[id]; ok {
		if n < max {
			return false
		} else {
			return true
		}
	}

	return false
}

func (ps *ProxyServer) In(id string) {
	ps.Lock()
	defer ps.Unlock()

	ps.tableIDtoMax[id]++
}

func (ps *ProxyServer) Out(id string) {
	ps.Lock()
	defer ps.Unlock()

	ps.tableIDtoMax[id]--
}

func (ps *ProxyServer) Wait() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	log.Printf("Ready... %s\n", ps.Cfg.ListenHostPort())

	select {
	case <-sigs:
	}

	log.Printf("Bye...\n")
}
