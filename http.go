package main

import (
	"time"

	"github.com/valyala/fasthttp"
)

var (
	Hc HTTPClient
)

func initHttpClientConfig() {
	client := &fasthttp.Client{
		MaxConnsPerHost:     workerNum * 2,
		ReadTimeout:         time.Second * time.Duration(timeoutSecond),
		WriteTimeout:        time.Second * time.Duration(timeoutSecond),
		MaxIdleConnDuration: 90 * time.Second,
	}
	if !useKeepAlive {
		client.MaxIdleConnDuration = 0

	}
	Hc = client
}
