package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"

	"gopkg.in/yaml.v2"
)

type BuffyConfig struct {
	Version   string         `json:"version"   yaml:"version"`
	Server    BuffyServerDef `json:"buffy"     yaml:"buffy"`
	Upstreams []UpstreamDef  `json:"upstreams" yaml:"upstreams"`
	Endpoints []EndpointDef  `json:"endpoints" yaml:"endpoints"`
}

type BuffyServerDef struct {
	Listen BuffyServerListen `json:"listen" yaml:"listen"`
	Admin  BuffyServerAdmin  `json:"admin"  yaml:"admin"`
}

type BuffyServerListen struct {
	Bind string `json:"bind" yaml:"bind"`
	Port int    `json:"port" yaml:"port"`
}

type BuffyServerAdmin struct {
	Path string `json:"path" yaml:"path"`
	Bind string `json:"bind" yaml:"bind"`
	Port int    `json:"port" yaml:"port"`
}

func (ed *EndpointDef) GetResponseWithName(name string) (string, error) {
	for _, e := range ed.Response {
		if e.Name == name {
			return e.Content, nil
		}
	}

	return "", errors.New("not found name")
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
