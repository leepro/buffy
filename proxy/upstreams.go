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
	GateClosed uint32 = iota
	GateOpened
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
	Id       string      `json:"id"       yaml:"id"`
	Endpoint string      `json:"endpoint" yaml:"endpoint"`
	Interval int         `json:"interval" yaml:"interval"`
	Autogate AutogateDef `json:"autogate" yaml:"autogate"`
}

type AutogateDef struct {
	Uri     string     `json:"uri"     yaml:"uri"`
	Matches []MatchDef `json:"matches" yaml:"matches"`
}

type MatchDef struct {
	Id   string `json:"id"   yaml:"id"`
	Type string `json:"type" yaml:"type"`
	If   string `json:"if"   yaml:"if"`
	Then string `json:"then" yaml:"then"`
}

type UpstreamHandler struct {
	ctx      context.Context
	def      *UpstreamDef
	revproxy *httputil.ReverseProxy
	notiC    chan string

	UpstreamStatus uint32 `json:"status"`
	GateState      uint32 `json:"gate"`

	sync.Mutex
}

func NewUpstream(ctx context.Context, u UpstreamDef, notiC chan string) (*Upstream, error) {
	up := &Upstream{
		Id:       u.Id,
		Endpoint: u.Endpoint,
		Def:      &u,
		Handler: &UpstreamHandler{
			ctx:            ctx,
			notiC:          notiC,
			def:            &u,
			UpstreamStatus: StatusNone,
			GateState:      GateOpened,
		},
	}

	go up.Handler.run()

	return up, nil
}

func (us *Upstream) Opengate() error {
	us.Handler.UpdateGate(GateOpened)
	return nil
}

func (us *Upstream) Closegate() error {
	us.Handler.UpdateGate(GateClosed)
	return nil
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
				_s := us.GetUpstreamStatus()
				if _s == StatusNone || _s == StatusAvailable {
					log.Printf("[upstream:%s/%d] Switch to 'Unavailable'\n", us.def.Id, cnt)
					us.notify(`{"status":"change". "desc":"upstream [` + us.def.Id + `] unavailable"}`)
				}
				us.UpdateUpstreamStatus(StatusUnavailable)
				continue
			}
			conn.Close()

			_s := us.GetUpstreamStatus()
			if _s == StatusNone || _s == StatusUnavailable {
				log.Printf("[upstream:%s/%d] Switch to 'Available'\n", us.def.Id, cnt)
				us.notify(`{"status":"change". "desc":"upstream [` + us.def.Id + `] available"}`)
			}

			us.UpdateUpstreamStatus(StatusAvailable)
		}
	}
}

func (us *UpstreamHandler) UpdateUpstreamStatus(s uint32) {
	atomic.StoreUint32(&us.UpstreamStatus, s)
}

func (us *UpstreamHandler) UpdateGate(g uint32) {
	atomic.StoreUint32(&us.GateState, g)
}

func (us *UpstreamHandler) GetUpstreamStatus() uint32 {
	return atomic.LoadUint32(&us.UpstreamStatus)
}

func (us *UpstreamHandler) GetGateState() uint32 {
	return atomic.LoadUint32(&us.GateState)
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
		getUpstreamStatus: up.Handler.GetUpstreamStatus,
		getGateState:      up.Handler.GetGateState,
	}
	// up.Handler.revproxy.ErrorHandler = func(http.ResponseWriter, *http.Request, error) {
	// }

	return nil
}

func (up *Upstream) Forward(w http.ResponseWriter, r *http.Request) {
	r.Header.Add("X-Buffy-Upstream-ID", up.Def.Id)
	up.Handler.revproxy.ServeHTTP(w, r)
}
