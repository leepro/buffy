package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/leepro/buffy/proxy"
)

var BuildVersion string

var (
	filename = flag.String("c", "", "config file")
	version  = flag.Bool("v", false, "versoin")
)

func main() {
	flag.Parse()

	if *version {
		fmt.Printf("buffy build:%s\n", BuildVersion)
		os.Exit(0)
	}

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
