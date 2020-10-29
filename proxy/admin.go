package proxy

import (
	"encoding/json"
	"net/http"
	"strings"
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

	path := strings.Replace(r.URL.Path, ps.Cfg.Server.Admin.Path+"/gate/", "", 1)
	params := strings.Split(path, "/")
	if len(params) != 2 {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid path"))
		return
	}

	upstreamId := params[0]
	action := params[1]

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
