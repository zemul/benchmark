package main

import (
	"flag"
	"runtime"
	"strings"
)

var (
	// base
	workerNum int
	requests  int
	cpuNum    int

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
	// base
	flag.IntVar(&cpuNum, "cpu", runtime.NumCPU()/2, "maximum number of cpus")
	flag.IntVar(&workerNum, "c", 1, "concurrent worker")
	flag.IntVar(&requests, "n", 0, "number of requests to perform")
	flag.IntVar(&timelimit, "t", 0, "seconds to max. to spend on benchmarking")

	// network
	flag.BoolVar(&useKeepAlive, "k", false, "Use KeepAlive")
	flag.Int64Var(&timeoutSecond, "s", 30, "Seconds to max. wait for each response")

	// request
	flag.StringVar(&method, "m", "GET", "Http call method, options[POST, PUT]")
	flag.Var(&headers, "H", "Custom header eg: 'Accept-Encoding: gzip' (can be repeated)")
	flag.IntVar(&fileSizeMin, "min", 10, "body minlength (byte)")
	flag.IntVar(&fileSizeMax, "max", 100, "body maxlength (byte)")
	flag.StringVar(&urlListFilePath, "f", "", "filePath: dataset filePath")

	flag.StringVar(&bodyFile, "b", "", "File containing data to Call")
	flag.StringVar(&bodyFile, "postfile", "", "File containing data to Call")

	flag.StringVar(&contentType, "T", "text/plain", "Content-type header to use for POST/PUT data, eg.'application/x-www-form-urlencoded'")
	flag.StringVar(&contentType, "content-type", "text/plain", "Content-type header to use for POST/PUT data, eg.'application/x-www-form-urlencoded'")

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
