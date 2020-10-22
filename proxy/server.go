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
	"time"
)

const (
	NameHitMaxQueue = "hit_max_queue"
	NameHitTimeout  = "hit_timeout"
	NameOK          = "200"
)

type ProxyServer struct {
	Cfg *BuffyConfig

	ServerBindAddr string
	AdminBindAddr  string

	upstreams []*Upstream
	endpoints []*Endpoint

	mux *http.ServeMux

	ctx       context.Context
	ctxCancel context.CancelFunc
	sync.Mutex
}

type Upstream struct {
	Id       string           `json:"id"`
	Endpoint string           `json:"endpoint"`
	Def      *UpstreamDef     `json:"-"`
	Handler  *UpstreamHandler `json:"handler"`
}

type Endpoint struct {
	Id      string           `json:"id"`
	Path    string           `json:"path"`
	Def     *EndpointDef     `json:"-"`
	Handler *EndpointHandler `json:"handler"`
}

func ListenAndServe(cfg *BuffyConfig) (*ProxyServer, error) {
	ctx, ctxCancel := context.WithCancel(context.Background())

	ps := &ProxyServer{
		Cfg:            cfg,
		ServerBindAddr: cfg.ServerListenHostPort(),
		AdminBindAddr:  cfg.AdminListenHostPort(),
		ctx:            ctx,
		ctxCancel:      ctxCancel,
		mux:            &http.ServeMux{},
	}

	if err := ps.RunServer(); err != nil {
		return nil, err
	}

	if err := ps.RunAdmin(); err != nil {
		return nil, err
	}

	return ps, nil
}

func (ps *ProxyServer) RunServer() error {
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
		Addr:    ps.ServerBindAddr,
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

func (ps *ProxyServer) RunAdmin() error {
	mux := http.NewServeMux()
	mux.HandleFunc(ps.Cfg.Server.Admin.Path+"/config", ps.AdminHandleConfig)
	mux.HandleFunc(ps.Cfg.Server.Admin.Path+"/status", ps.AdminHandleStatus)

	srv := &http.Server{
		Addr:    ps.AdminBindAddr,
		Handler: mux,
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
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte("{ \"status\": \"not implemented\"}"))
}

func (ps *ProxyServer) CreateUpstreamHandlers() error {
	for _, u := range ps.Cfg.Upstreams {
		up, err := NewUpstream(ps.ctx, u)
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
		endp, err := NewEndpoint(ps.ctx, epdef)
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

	log.Printf("Ready... server: %s\n", ps.Cfg.ServerListenHostPort())
	log.Printf("Ready...  admin: %s\n", ps.Cfg.AdminListenHostPort())

	select {
	case <-sigs:
		ps.ctxCancel()
	}

	time.Sleep(2 * time.Second)

	log.Printf("Bye...\n")
}
