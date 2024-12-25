package main

import (
	"flag"
	"os"
)

var (
	workerNum       int
	fileSizeMin     int
	fileSizeMax     int
	write           bool
	read            bool
	delete          bool
	urlListFilePath string
	bodyPath        string
	contentType     string
	requests        int
	timelimit       int
	path            string
	param           string
	files           int
	cpuNum          int
	body            []byte
)

func init() {
	flag.IntVar(&requests, "n", 0, "number of requests to perform")
	flag.IntVar(&timelimit, "t", 0, "seconds to max. to spend on benchmarking")
	flag.IntVar(&fileSizeMin, "min", 10, "body minlength (byte)")
	flag.IntVar(&fileSizeMax, "max", 100, "body maxlength (byte)")
	flag.StringVar(&urlListFilePath, "f", os.TempDir()+"/benchmark_list.txt", "filePath: dataset filePath")
	flag.StringVar(&bodyPath, "b", "", "body file path")
	flag.StringVar(&contentType, "contentType", "multipart/form-data", "Http call contentType, options[text/plain, application/json, multipart/form-data]")
}
