package main

import (
	"net/http"
	"time"
)

var (
	Hc HTTPClient
)

func initHttpClientConfig() {
	Hc = &http.Client{Transport: &http.Transport{
		MaxIdleConns:        1024,
		MaxIdleConnsPerHost: 1024,
		DisableKeepAlives:   !useKeepAlive,
	}, Timeout: time.Second * time.Duration(timeoutSecond)}
}
