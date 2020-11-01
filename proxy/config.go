package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

type BuffyConfig struct {
	Version        string        `json:"version"   yaml:"version"`
	Server         ServerDef     `json:"buffy"     yaml:"buffy"`
	Upstreams      []UpstreamDef `json:"upstreams" yaml:"upstreams"`
	Endpoints      []EndpointDef `json:"endpoints" yaml:"endpoints"`
	ConfigFilename string        `json:"filename"`
	BasePath       string        `json:"basepath"`
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

func (ed *EndpointDef) GetResponseWithName(name string, basepath string) (int, string, error) {
	for _, e := range ed.Response {
		if e.Name == name {
			if strings.HasPrefix(e.Content, "file://") {
				content, err := ed.ReadContentFile(e.Content, basepath)
				if err != nil {
					return http.StatusInternalServerError, "", err
				}
				return e.ReturnCode, content, err
			} else {
				return e.ReturnCode, e.Content, nil
			}
		}
	}

	return http.StatusInternalServerError, "", errors.New("not found name")
}

func (ed *EndpointDef) ReadContentFile(filename string, basepath string) (string, error) {
	u, err := url.ParseRequestURI(filename)
	if err != nil {
		return "", err
	}

	content, err := ioutil.ReadFile(basepath + "/" + u.Path)
	return string(content), err
}

func ReadConfigFile(filename string) (*BuffyConfig, error) {
	bs, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var t BuffyConfig
	err = yaml.Unmarshal(bs, &t)

	t.ConfigFilename, _ = filepath.Abs(filename)
	t.BasePath = filepath.Dir(t.ConfigFilename)

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
	log.Printf("- config    : %s\n", cfg.ConfigFilename)
	log.Printf("- base      : %s\n", cfg.BasePath)
	log.Printf("- server    : %s:%d\n", cfg.Server.Listen.Bind, cfg.Server.Listen.Port)
	log.Printf("- admin     : %s:%d\n", cfg.Server.Admin.Bind, cfg.Server.Admin.Port)
	log.Printf("- webhook   : '%s'\n", cfg.Server.Admin.Notify.Webhook)
	log.Printf("- slack     : '%s'\n", cfg.Server.Admin.Notify.Slack)

	log.Printf("- upstreams : %d\n", len(cfg.Upstreams))
	for _, up := range cfg.Upstreams {
		log.Printf("  - %s: %s\n", up.Id, up.Endpoint)
	}

	log.Printf("- endpoints : %d\n", len(cfg.Endpoints))
	for _, ep := range cfg.Endpoints {
		log.Printf("  - %s: %s timeout:%d queue:%d mode:%s\n", ep.Id, ep.Path, ep.Timeout, ep.MaxQueue, ep.ProxyMode)
	}

	log.Println()
}

func (cfg *BuffyConfig) ServerListenHostPort() string {
	return fmt.Sprintf("%s:%d", cfg.Server.Listen.Bind, cfg.Server.Listen.Port)
}

func (cfg *BuffyConfig) AdminListenHostPort() string {
	return fmt.Sprintf("%s:%d", cfg.Server.Admin.Bind, cfg.Server.Admin.Port)
}
