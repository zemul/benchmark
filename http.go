package main

import (
	"flag"
	"net/http"
	"time"
)

var (
	Hc HTTPClient
)

var (
	useKeepAlive  bool
	timeoutSecond int64
)

func init() {
	flag.BoolVar(&useKeepAlive, "k", false, "使用KeepAlive功能")
	flag.Int64Var(&timeoutSecond, "s", 30, "请求超时时间(秒)")
}

func initHttpClientConfig() {
	Hc = &http.Client{Transport: &http.Transport{
		MaxIdleConns:        1024,
		MaxIdleConnsPerHost: 1024,
		DisableKeepAlives:   !useKeepAlive,
	}, Timeout: time.Second * time.Duration(timeoutSecond)}
}
