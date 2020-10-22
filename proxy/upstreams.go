package proxy

import (
	"context"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

const (
	StatusNone = iota
	StatusUnavailable
	StatusAvailable
)

const (
	DefaultIntervalPing = 2 * time.Second
	TimeoutTCPDialCheck = 1 * time.Second
)

type UpstreamDef struct {
	Id       string `json:"id"       yaml:"id"`
	Endpoint string `json:"endpoint" yaml:"endpoint"`
	Interval int    `json:"interval" yaml:"interval"`
}

type UpstreamResponseDef struct {
	Name     string `json:"name"     yaml:"name"`
	Content  string `json:"content"  yaml:"content"`
	Interval string `json:"interval" yaml:"interval"`
}

type UpstreamHandler struct {
	ctx      context.Context
	def      *UpstreamDef
	revproxy *httputil.ReverseProxy
	notiC    chan string

	Status int `json:"status"`
}

func NewUpstream(ctx context.Context, u UpstreamDef, notiC chan string) (*Upstream, error) {
	up := &Upstream{
		Id:       u.Id,
		Endpoint: u.Endpoint,
		Def:      &u,
		Handler: &UpstreamHandler{
			ctx:    ctx,
			notiC:  notiC,
			def:    &u,
			Status: StatusNone,
		},
	}

	go up.Handler.run()

	return up, nil
}

func (us *UpstreamHandler) run() {
	var tick *time.Ticker
	if us.def.Interval == 0 {
		tick = time.NewTicker(DefaultIntervalPing)
	} else {
		tick = time.NewTicker(time.Duration(us.def.Interval) * time.Millisecond)
	}
	defer tick.Stop()

	cnt := 0
	for {
		select {
		case <-us.ctx.Done():
			log.Printf("[upstream:%s] cancelled\n", us.def.Id)
			return

		case <-tick.C:
			cnt++

			u, _ := url.Parse(us.def.Endpoint)
			d := net.Dialer{Timeout: TimeoutTCPDialCheck}
			conn, err := d.Dial("tcp", u.Host)
			if err != nil {
				if us.Status == StatusNone || us.Status == StatusAvailable {
					log.Printf("[upstream:%s/%d] Switch to 'Unavailable'\n", us.def.Id, cnt)
					us.notify(`{"status":"change". "desc":"upstream [` + us.def.Id + `] unavailable"}`)
				}
				us.Status = StatusUnavailable
				continue
			}
			conn.Close()

			if us.Status == StatusNone || us.Status == StatusUnavailable {
				log.Printf("[upstream:%s/%d] Switch to 'Available'\n", us.def.Id, cnt)
				us.notify(`{"status":"change". "desc":"upstream [` + us.def.Id + `] available"}`)
			}

			us.Status = StatusAvailable
		}
	}
}

func (us *UpstreamHandler) notify(msg string) {
	if len(us.notiC) < cap(us.notiC) {
		us.notiC <- msg
	}
}

func (up *Upstream) CreateReverseProxy(timeout int) error {
	upURL, err := url.Parse(up.Def.Endpoint)
	if err != nil {
		return err
	}

	up.Handler.revproxy = httputil.NewSingleHostReverseProxy(upURL)
	up.Handler.revproxy.Transport = &MyTransport{timeout: timeout}

	return nil
}

func (up *Upstream) Forward(w http.ResponseWriter, r *http.Request) {
	up.Handler.revproxy.ServeHTTP(w, r)
}
