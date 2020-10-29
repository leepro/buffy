package proxy

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

const (
	StatusNone uint32 = iota
	StatusUnavailable
	StatusAvailable
)

const (
	DefaultIntervalPing = 2 * time.Second
	TimeoutTCPDialCheck = 1 * time.Second
)

const (
	ProxyModeStoreAndForward = "store_and_forward"
	ProxyModeBypass          = "bypass"
)

type UpstreamDef struct {
	Id       string `json:"id"       yaml:"id"`
	Endpoint string `json:"endpoint" yaml:"endpoint"`
	Interval int    `json:"interval" yaml:"interval"`
}

type UpstreamHandler struct {
	ctx      context.Context
	def      *UpstreamDef
	revproxy *httputil.ReverseProxy
	notiC    chan string

	Status uint32 `json:"status"`

	sync.Mutex
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
				_s := us.GetStatus()
				if _s == StatusNone || _s == StatusAvailable {
					log.Printf("[upstream:%s/%d] Switch to 'Unavailable'\n", us.def.Id, cnt)
					us.notify(`{"status":"change". "desc":"upstream [` + us.def.Id + `] unavailable"}`)
				}
				us.UpdateStatus(StatusUnavailable)
				continue
			}
			conn.Close()

			_s := us.GetStatus()
			if _s == StatusNone || _s == StatusUnavailable {
				log.Printf("[upstream:%s/%d] Switch to 'Available'\n", us.def.Id, cnt)
				us.notify(`{"status":"change". "desc":"upstream [` + us.def.Id + `] available"}`)
			}

			us.UpdateStatus(StatusAvailable)
		}
	}
}

func (us *UpstreamHandler) UpdateStatus(s uint32) {
	atomic.StoreUint32(&us.Status, s)
}

func (us *UpstreamHandler) GetStatus() uint32 {
	return atomic.LoadUint32(&us.Status)
}

func (us *UpstreamHandler) notify(msg string) {
	if len(us.notiC) < cap(us.notiC) {
		us.notiC <- msg
	}
}

func (up *Upstream) CreateReverseProxy(mode string, timeout int) error {
	upURL, err := url.Parse(up.Def.Endpoint)
	if err != nil {
		return err
	}

	switch mode {
	case ProxyModeStoreAndForward:
	case ProxyModeBypass:
	default:
		return errors.New("invalid proxy mode")
	}

	up.Handler.revproxy = httputil.NewSingleHostReverseProxy(upURL)
	up.Handler.revproxy.Transport = &MyTransport{
		upstream:          up.Id,
		mode:              mode,
		timeout:           timeout,
		interval:          up.Def.Interval,
		getUpstreamStatus: up.Handler.GetStatus,
	}
	// up.Handler.revproxy.ErrorHandler = func(http.ResponseWriter, *http.Request, error) {
	// }

	return nil
}

func (up *Upstream) Forward(w http.ResponseWriter, r *http.Request) {
	r.Header.Add("X-Buffy-Upstream-ID", up.Def.Id)
	up.Handler.revproxy.ServeHTTP(w, r)
}
