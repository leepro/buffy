package proxy

import (
	"encoding/json"
	"net/http"
)

const (
	ActionOpen  = "open"
	ActionClose = "close"
)

func (ps *ProxyServer) AdminHandleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	bs, _ := json.Marshal(ps.Cfg)
	w.Write(bs)
}

func (ps *ProxyServer) AdminHandleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")

	ps.Lock()
	defer ps.Unlock()

	ret := map[string]interface{}{
		"server":    ps.Cfg.Server,
		"upstreams": ps.upstreams,
		"endpoints": ps.endpoints,
	}

	bs, _ := json.Marshal(ret)
	w.Write(bs)
}

func (ps *ProxyServer) AdminHandleGate(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")

	upstreamId := r.URL.Query().Get("upstream")
	action := r.URL.Query().Get("action")

	if upstreamId != "" || action != "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid parameters"))
		return
	}

	ps.Lock()
	defer ps.Unlock()

	var u *Upstream
	var err error

	if u, err = ps.LookupUpstreamWithIds([]string{upstreamId}); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("not found upstream id"))
		return
	}

	switch action {
	case ActionOpen:
		if err := u.Opengate(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("failed to control the gate: " + err.Error()))
			return
		}
	case ActionClose:
		if err := u.Closegate(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("failed to control the gate: " + err.Error()))
			return
		}
	default:
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid action"))
		return
	}

	ret := map[string]interface{}{
		"status":   "ok",
		"upstream": upstreamId,
		"action":   action,
	}

	bs, _ := json.Marshal(ret)
	w.Write(bs)
}
