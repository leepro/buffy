package proxy

import "errors"

const (
	TypeRespond = "respond"
	TypeProxy   = "proxy"
)

type EndpointHandler struct {
}

func NewEndpoint(e EndpointDef) (*Endpoint, error) {
	ep := &Endpoint{
		Def:     &e,
		Handler: &EndpointHandler{},
	}
	return ep, nil
}

func (ed *EndpointDef) GetResponseWithName(name string) (string, error) {
	for _, e := range ed.Response {
		if e.Name == name {
			return e.Content, nil
		}
	}

	return "", errors.New("not found name")
}
