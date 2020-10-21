package proxy

import (
	"fmt"
	"testing"

	"github.com/davecgh/go-spew/spew"
)

func TestConfigFile(t *testing.T) {
	cfg, err := ReadConfigFile("../examples/buffy.yaml")
	if err != nil {
		t.Error(err)
		return
	}

	spew.Dump(cfg)

	fmt.Printf("%s\n", cfg.JSON())
}
