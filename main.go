package main

import (
	"flag"
	"fmt"
	"os"

	log "github.com/Sirupsen/logrus"
	dknet "github.com/docker/go-plugins-helpers/network"
	"github.com/iavael/docker-ovs-plugin/ovs"
)

const (
	version = "0.2"
)

var (
	debug  bool
	proto  string
	target string
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.BoolVar(&debug, "debug", false, "enable debugging")
	flag.StringVar(&proto, "proto", "tcp", "ovsdb protocol")
	flag.StringVar(&target, "target", "localhost:6640", "ovsdb target")
	flag.Parse()
}

func main() {
	if debug {
		log.SetLevel(log.DebugLevel)
	}

	drv, err := ovs.NewDriver(proto, target)
	if err != nil {
		panic(err)
	}
	h := dknet.NewHandler(drv)
	h.ServeUnix("root", "ovs")
}
