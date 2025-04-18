package main

import (
	"flag"
	"math/rand"
	"strings"
	"time"
)

var (
	// base
	workerNum int
	requests  int
	cpuNum    int
	seed      = rand.New(rand.NewSource(time.Now().UnixNano()))

	// network
	useKeepAlive  bool
	timeoutSecond int64

	// req
	urlPath string
	method  string
	headers arrayFlags

	urlListFilePath string
	fileSizeMin     int
	fileSizeMax     int
	bodyFile        string
	contentType     string
	timelimit       int
	body            []byte
)

func init() {
	// io密集任务无需开启多核，避免线程切换
	// nginx等高性能服务开启多核可能测试结果更好
	flag.IntVar(&cpuNum, "cpu", 1, "maximum number of CPUs")
	flag.IntVar(&workerNum, "c", 1, "concurrent worker")
	flag.IntVar(&requests, "n", 0, "number of requests to perform")
	flag.IntVar(&timelimit, "t", 0, "seconds to max. to spend on benchmarking")

	// network
	flag.BoolVar(&useKeepAlive, "k", false, "Use KeepAlive")
	flag.Int64Var(&timeoutSecond, "s", 30, "Seconds to max. wait for each response")

	// request
	flag.StringVar(&method, "m", "GET", "Http call method, options[HEAD,GET,POST,PUT,DELETE]")
	flag.Var(&headers, "H", "Custom header eg: 'Accept-Encoding: gzip'")
	flag.IntVar(&fileSizeMin, "min", 10, "body minlength (byte)")
	flag.IntVar(&fileSizeMax, "max", 100, "body maxlength (byte)")
	flag.StringVar(&urlListFilePath, "f", "", "filePath: dataset filePath")

	flag.StringVar(&bodyFile, "b", "", "File containing data to Call")

	flag.StringVar(&contentType, "content-type", "", "Content-type header to use for POST/PUT data, eg.'application/x-www-form-urlencoded'")
}

var headerCache map[string]string

func GetHeader() map[string]string {
	if headerCache != nil {
		return headerCache
	}

	headerCache = make(map[string]string)
	for _, header := range headers {
		parts := strings.Split(header, ":")
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			headerCache[key] = value
		}
	}
	if contentType != "" {
		headerCache["Content-Type"] = contentType
	}
	if !useKeepAlive {
		headerCache["Connection"] = "close"
	}
	return headerCache
}

func checkRequiredFlags() {
	if timelimit == 0 && requests == 0 {
		panic("Invalid requests")
	}
	if cpuNum == 0 {
		panic("Invalid cpuNum")
	}
	if workerNum == 0 {
		panic("Invalid workerNum")
	}
	if urlPath == "" && urlListFilePath == "" {
		panic("Invalid urlPath or urlListFilePath")
	}
	if urlPath != "" && urlListFilePath != "" {
		panic("Repeat urlPath and urlListFilePath")
	}
}
