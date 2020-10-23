package main

import (
	"flag"
	"log"

	"github.com/leepro/buffy/proxy"
)

var (
	filename = flag.String("c", "", "config file")
)

func main() {
	flag.Parse()

	if *filename == "" {
		log.Fatal("no config yaml file specified")
	}

	cfg, err := proxy.ReadConfigFile(*filename)
	if err != nil {
		log.Fatalf("failed: %s\n", err)
	}

	cfg.ShowInfo()

	srv, err := proxy.ListenAndServe(cfg)
	if err != nil {
		log.Fatalf("failed: %s\n", err)
	}

	srv.Wait()
}
