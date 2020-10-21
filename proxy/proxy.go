package proxy

import (
	"encoding/json"
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
	Bind string `json:"bind" yaml:"bind"`
	Port int    `json:"port" yaml:"port"`
}

type UpstreamDef struct {
	Id       string `json:"id"       yaml:"id"`
	Endpoint string `json:"endpoint" yaml:"endpoint"`
}

type UpstreamResponseDef struct {
	Name     string `json:"name"     yaml:"name"`
	Content  string `json:"content"  yaml:"content"`
	Interval string `json:"interval" yaml:"interval"`
}

type EndpointDef struct {
	Id       string                `json:"id"        yaml:"id"`
	Desc     string                `json:"desc"      yaml:"desc"`
	Path     string                `json:"path"      yaml:"path"`
	Type     string                `json:"type"      yaml:"type"`
	Upstream []string              `json:"upstream"  yaml:"upstream"`
	Timeout  int                   `json:"timeout"   yaml:"timeout"`
	MaxQueue int                   `json:"max_queue" yaml:"max_queue"`
	Methods  []string              `json:"methods"   yaml:"methods"`
	Response []UpstreamResponseDef `json:"response"  yaml:"response"`
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

func (cfg *BuffyConfig) ListenHostPort() string {
	return fmt.Sprintf("%s:%d", cfg.Server.Bind, cfg.Server.Port)
}
