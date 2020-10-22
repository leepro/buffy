package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"strings"

	"gopkg.in/yaml.v2"
)

type BuffyConfig struct {
	Version   string        `json:"version"   yaml:"version"`
	Server    ServerDef     `json:"buffy"     yaml:"buffy"`
	Upstreams []UpstreamDef `json:"upstreams" yaml:"upstreams"`
	Endpoints []EndpointDef `json:"endpoints" yaml:"endpoints"`
}

type ServerDef struct {
	Listen ServerListen `json:"listen" yaml:"listen"`
	Admin  ServerAdmin  `json:"admin"  yaml:"admin"`
}

type ServerListen struct {
	Bind string `json:"bind" yaml:"bind"`
	Port int    `json:"port" yaml:"port"`
}

type ServerAdmin struct {
	Path   string      `json:"path"    yaml:"path"`
	Bind   string      `json:"bind"    yaml:"bind"`
	Port   int         `json:"port"    yaml:"port"`
	Notify AdminNotify `json:"notify"  yaml:"notify"`
}
type AdminNotify struct {
	Webhook string `json:"webhook" yaml:"webhook"`
	Slack   string `json:"slack"   yaml:"slack"`
}

func (ed *EndpointDef) GetResponseWithName(name string) (string, error) {
	for _, e := range ed.Response {
		if e.Name == name {
			if strings.HasPrefix(e.Content, "file://") {
				content, err := ed.ReadContentFile(e.Content)
				return content, err
			} else {
				return e.Content, nil
			}
		}
	}

	return "", errors.New("not found name")
}

func (ed *EndpointDef) ReadContentFile(filename string) (string, error) {
	u, err := url.ParseRequestURI(filename)
	if err != nil {
		return "", err
	}

	fmt.Printf("url:%v\nscheme:%v host:%v Path:%v\n\n", u, u.Scheme, u.Host, u.Path)
	content, err := ioutil.ReadFile(u.Path)
	return string(content), err
}

func ReadConfigFile(filename string) (*BuffyConfig, error) {
	bs, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var t BuffyConfig
	err = yaml.Unmarshal(bs, &t)
	return &t, err
}

func (cfg *BuffyConfig) JSON() string {
	bs, _ := json.MarshalIndent(cfg, "", "\t")
	return string(bs)
}

func (cfg *BuffyConfig) ShowInfo() {
	log.Printf("* Buffy 1.0\n")
	log.Println()
	log.Printf("- version   : %s\n", cfg.Version)
	log.Printf("- server    : %s\n", cfg.Server.Listen.Bind)
	log.Printf("- admin     : %s\n", cfg.Server.Admin.Bind)
	log.Printf("- webhook   : '%s'\n", cfg.Server.Admin.Notify.Webhook)
	log.Printf("- slack     : '%s'\n", cfg.Server.Admin.Notify.Slack)
	log.Printf("- upstreams : %d\n", len(cfg.Upstreams))
	for _, up := range cfg.Upstreams {
		log.Printf("  - %s: %s\n", up.Id, up.Endpoint)
	}

	log.Printf("- endpoints : %d\n", len(cfg.Endpoints))
	for _, ep := range cfg.Endpoints {
		log.Printf("  - %s: %s\n", ep.Id, ep.Path)
	}

	log.Println()
}

func (cfg *BuffyConfig) ServerListenHostPort() string {
	return fmt.Sprintf("%s:%d", cfg.Server.Listen.Bind, cfg.Server.Listen.Port)
}

func (cfg *BuffyConfig) AdminListenHostPort() string {
	return fmt.Sprintf("%s:%d", cfg.Server.Admin.Bind, cfg.Server.Admin.Port)
}
