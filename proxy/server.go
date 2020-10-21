package proxy

import (
	"context"
	"errors"
	"log"
	"net/http"
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

	upstreams []*Upstream
	endpoints []*Endpoint

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
		Cfg:  cfg,
		Bind: cfg.ListenHostPort(),
		ctx:  context.Background(),
		mux:  &http.ServeMux{},
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

func (ps *ProxyServer) LookupUpstreamWithIds(ids []string) (*Upstream, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	for _, u := range ps.upstreams {
		if u.Def.Id == ids[0] {
			return u, nil
		}
	}
	return nil, errors.New("not found upstream with id: " + ids[0])
}

func (ps *ProxyServer) RegisterEndpoints() error {
	for _, epdef := range ps.Cfg.Endpoints {
		endp, err := NewEndpoint(epdef)
		if err != nil {
			return err
		}
		ps.endpoints = append(ps.endpoints, endp)

		upstream, err := ps.LookupUpstreamWithIds(epdef.Upstream)
		if err != nil {
			return err
		}

		if err := endp.Handler.RegisterRoute(ps.mux, upstream); err != nil {
			return err
		}
	}
	return nil
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
