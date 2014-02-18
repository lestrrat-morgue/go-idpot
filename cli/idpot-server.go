package main

import (
  "flag"
  "log"
  "code.google.com/p/gcfg"
  "github.com/lestrrat/go-idpot/server"
)

func main() {
  var cfgfile  string
  var listen string

  flag.StringVar(&cfgfile, "config", "idpot.gcfg", "config file gcfg format")
  flag.StringVar(&listen, "listen", "", "listen address")
  flag.Parse()

  opts := &server.ServerOpts {}
  cfg := struct {
    Server server.ServerOpts
    Mysql  server.MysqlServer
  } {}
  if cfgfile != "" {
    err := gcfg.ReadFileInto(&cfg, cfgfile)
    if err != nil {
      log.Fatalf("Failed to read config file %s: %s", cfgfile, err)
    }

    opts = &cfg.Server
    opts.Mysql = &cfg.Mysql
  }

  if listen != "" {
    opts.Listen = listen
  }

  s := server.New(opts)
  s.Start()
}