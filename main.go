package main

import (
	"bufio"
	flag "flag"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// test upload filer time
/*
1. upload to filer
2. upload to proxy

1. 5G*1 thread=1
2. 50KB*10000 thread=50
*/

// seq 1000000 | xargs -i dd if=/dev/zero of={}.dat bs=1024 count=1
/**
client:
  dialTimeout: 5
  keepalive: 30
  maxIdleConn: 100
  maxConnPerHost: 50
  maxIdleConnPerHost: 50
  idleConnTimeout: 90
  tlsTimeout: 10
  # https 跳过证书验证
  insecureSkipVerify: true
*/

/*
filer
POST,"http://10.17.100.28:8888/buckets/bechmark/x.dat"
*/

var workerNum int
var fileSizeMin int
var fileSizeMax int
var write bool
var read bool
var delete bool
var urlListFilePath string
var contentType string
var requests int

var path string
var param string
var files int
var cpuNum int

var (
	wait  sync.WaitGroup
	Stats *stats

	Addrs  []Msg
	Addr   string
	Method string
)

func main() {
	log.SetFlags(log.Llongfile | log.Lmicroseconds | log.Ldate)
	flag.IntVar(&cpuNum, "cpu", runtime.NumCPU()/2, "maximum number of CPUs")
	flag.IntVar(&workerNum, "c", 1, "concurrent worker")
	flag.IntVar(&requests, "n", 0, "number of requests to perform")

	flag.IntVar(&fileSizeMin, "min", 10, "body minlength (byte)")
	flag.IntVar(&fileSizeMax, "max", 100, "body maxlength (byte)")

	//flag.StringVar(&Addr, "a", "", "call addr")
	//flag.StringVar(&Method, "x", "GET", "call method")
	flag.StringVar(&urlListFilePath, "f", os.TempDir()+"/benchmark_list.txt", "filePath: dataset filePath")
	flag.StringVar(&contentType, "contentType", "multipart/form-data", "Http call contentType, options[text/plain, application/json, multipart/form-data]")

	//flag.StringVar(&path, "genPath", "", "Generating HTTP addresses")
	//flag.IntVar(&files, "genNum", 0, "Generating num")
	//flag.StringVar(&param, "genParam", "", "replication=000")
	flag.Parse()
	runtime.GOMAXPROCS(cpuNum)
	//if files > 0 && path != "" {
	//	GenFile(files)
	//	return
	//}
	benchTest()

}

func benchTest() {
	finishChan := make(chan bool)
	pathChan := make(chan Msg, 100)
	Stats = newStats(workerNum)
	go ReadFileIds(urlListFilePath, pathChan, Stats)
	for i := 0; i < workerNum; i++ {
		wait.Add(1)
		go ThreadTask(pathChan, i)
	}
	Stats.start = time.Now()
	go Stats.checkProgress("Benchmark", finishChan)
	wait.Wait()
	Stats.end = time.Now()
	wait.Add(1)
	finishChan <- true
	wait.Wait()
	close(finishChan)
	Stats.printStats()
	Stats.printStatsWithMethod(http.MethodHead)
	Stats.printStatsWithMethod(http.MethodGet)
	Stats.printStatsWithMethod(http.MethodPost)
	Stats.printStatsWithMethod(http.MethodPut)
	Stats.printStatsWithMethod(http.MethodDelete)

}

func ThreadTask(pathChan chan Msg, idx int) {
	defer wait.Done()
	random := rand.New(rand.NewSource(time.Now().UnixNano()))
	var reqLen, respLen int
	var err error
	for row := range pathChan {
		reqLen, respLen, err = 0, 0, nil
		start := time.Now()
		switch row.method {
		case http.MethodGet:
			{
				reqLen, respLen, _, err = Get(row.url)
			}
		case http.MethodPost:
			{
				size := int64(fileSizeMin + random.Intn(fileSizeMax-fileSizeMin))
				reader := &FakeReader{id: uint64(rand.Uint64()), size: size, random: random}
				reqLen, respLen, err = Upload(reader, &UploadOption{
					Method:    row.method,
					UploadUrl: row.url,
					Filename:  filepath.Base(row.url),
					MimeType:  contentType,
				})
			}
		case http.MethodPut:
			{
				size := int64(fileSizeMin + random.Intn(fileSizeMax-fileSizeMin))
				reader := &FakeReader{id: uint64(rand.Uint64()), size: size, random: random}
				reqLen, respLen, err = Upload(reader, &UploadOption{
					Method:    row.method,
					UploadUrl: row.url,
					Filename:  filepath.Base(row.url),
					MimeType:  contentType,
				})
			}
		case http.MethodDelete:
			{
				reqLen, respLen, err = Delete(row.url)

			}
		case http.MethodHead:
			{
				reqLen, respLen, err = Head(row.url)
			}
		}
		if err == nil {
			Stats.localStats[row.method][idx].completed++
			Stats.localStats[row.method][idx].reqtransfer += reqLen
			Stats.localStats[row.method][idx].resptransfer += respLen
		} else {
			Stats.localStats[row.method][idx].failed++
		}
		Stats.addSample(row.method, time.Now().Sub(start))

	}
}

func ReadFileIds(urlListFilePath string, pathChan chan Msg, stats *stats) {

	Addrs = []Msg{}
	f, err := os.Open(urlListFilePath)
	if err != nil {
		log.Fatalf("File to read file %s: %s\n", urlListFilePath, err)
	}
	defer f.Close()
	r := bufio.NewReader(f)
	for {
		line, _, err := r.ReadLine()
		if err != nil || err == io.EOF {
			break
		}
		raw := strings.Split(string(line), ",")
		if len(raw) < 2 {
			panic("illegal url")
		}
		msg := Msg{method: strings.ToUpper(raw[0]),
			url: strings.Join(raw[1:], ""),
		}
		Addrs = append(Addrs, msg)
		stats.total += 1
	}

	if requests > 0 {
		stats.total = requests
	}

	for curr, idx := 0, 0; curr < stats.total; {
		if idx >= len(Addrs) {
			idx = 0
		}
		pathChan <- Addrs[idx]
		idx++
		curr++
	}
	close(pathChan)
}

func GenFile(num int) {
	f, err := os.OpenFile(os.TempDir()+"/benchmark_list.txt", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatalf("File to create file %s: %s\n", "benchmark_list.txt", err)
	}
	defer f.Close()
	for i := 0; i < num; i++ {
		f.Write([]byte(path))
		f.Write([]byte("/" + strconv.Itoa(i)))
		if param != "" {
			f.Write([]byte("?" + param))
		}
		f.Write([]byte("\n"))
	}

}
