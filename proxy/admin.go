package proxy

import (
	"encoding/json"
	"net/http"
)

func (ps *ProxyServer) AdminHandleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	bs, _ := json.Marshal(ps.Cfg)
	w.Write(bs)
}

func (ps *ProxyServer) AdminHandleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")

	ps.Lock()
	ret := map[string]interface{}{
		"upstreams": ps.upstreams,
		"endpoints": ps.endpoints,
	}
	bs, _ := json.Marshal(ret)
	w.Write(bs)
	ps.Unlock()
}
