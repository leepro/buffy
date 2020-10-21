package proxy

type UpstreamHandler struct {
}

func NewUpstream(u UpstreamDef) (*Upstream, error) {
	up := &Upstream{
		Def:     &u,
		Handler: &UpstreamHandler{},
	}

	return up, nil
}
