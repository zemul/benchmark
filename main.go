package main

import (
	"bufio"
	"bytes"
	flag "flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/valyala/fasthttp"
)

var (
	wait  sync.WaitGroup
	Stats *stats
)

func main() {
	log.SetFlags(log.Llongfile | log.Lmicroseconds | log.Ldate)
	flag.Parse()
	if len(flag.Args()) > 0 {
		urlPath = flag.Args()[0]
	}

	checkRequiredFlags()
	runtime.GOMAXPROCS(cpuNum)

	initHttpClientConfig()

	benchTest()
}

func benchTest() {
	var err error
	if bodyFile != "" {
		body, err = os.ReadFile(bodyFile)
		if err != nil {
			panic(fmt.Errorf("read body file: %v", err))
		}
	}

	finishChan := make(chan bool)
	pathChan := make(chan Msg, workerNum*2)
	cancelChan := make(chan struct{})
	Stats = newStats(workerNum * 2)
	go ReadFileIds(pathChan, cancelChan, Stats)
	for i := 0; i < workerNum; i++ {
		wait.Add(1)
		go ThreadTask(pathChan, i)
	}
	Stats.start = time.Now()
	go Stats.checkProgress("Benchmark", finishChan)

	done := make(chan struct{})
	go func() {
		wait.Wait()
		close(done)
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	select {
	case <-done:
	case <-sigChan:
		close(cancelChan)
	}

	Stats.end = time.Now()
	wait.Add(1)
	select {
	case finishChan <- true:
		wait.Wait()
		close(finishChan)
	default:
		wait.Done()
	}

	if bodyFile != "" {
		Stats.printStats()
	}
	Stats.printStatsWithMethod(http.MethodHead)
	Stats.printStatsWithMethod(http.MethodGet)
	Stats.printStatsWithMethod(http.MethodPost)
	Stats.printStatsWithMethod(http.MethodPut)
	Stats.printStatsWithMethod(http.MethodDelete)
}

func ThreadTask(pathChan chan Msg, idx int) {
	defer wait.Done()

	for row := range pathChan {
		start := time.Now()
		resp, err := processRequest(row)
		if err == nil {
			length := resp.Header.ContentLength()
			updateStats(row.method, idx, resp, length)
		} else {
			Stats.localStats[row.method][idx].failed++
		}

		CloseResponse(resp)
		Stats.addSample(row.method, idx, time.Since(start))
	}
}

func processRequest(row Msg) (*fasthttp.Response, error) {
	switch row.method {
	case "GET":
		return Get(row.url)
	case "HEAD":
		return Head(row.url)
	case "DELETE":
		return Delete(row.url)
	case "POST", "PUT":
		return handleUpload(row)
	default:
		return nil, fmt.Errorf("unsupported method: %s", row.method)
	}
}

func handleUpload(row Msg) (*fasthttp.Response, error) {
	var reader io.Reader
	if bodyFile != "" {
		reader = bytes.NewReader(body)
	} else {
		size := int64(fileSizeMin + seed.Intn(fileSizeMax-fileSizeMin))
		reader = &FakeReader{
			id:     uint64(rand.Uint64()),
			size:   size,
			random: seed,
		}
	}

	return Upload(reader, &CallOption{
		Method:    row.method,
		UploadUrl: row.url,
		Filename:  filepath.Base(row.url),
		MimeType:  contentType,
	})
}

func updateStats(method string, idx int, resp *fasthttp.Response, length int) {
	atomic.AddInt64(&Stats.localStats[method][idx].completed, 1)
	Stats.localStats[method][idx].resptransfer += length
	if resp.StatusCode() > 299 {
		Stats.localStats[method][idx].not2xx++
		Stats.localStats[method][idx].failed++
	}
}

func ReadFileIds(pathChan chan Msg, cancelChan chan struct{}, stats *stats) {
	if urlPath != "" {
		readSingleUrl(pathChan, cancelChan, stats)
		return
	}
	readUrlsFromFile(pathChan, cancelChan, stats)
}

func readSingleUrl(pathChan chan Msg, cancelChan chan struct{}, stats *stats) {
	msg := Msg{
		method: method,
		url:    urlPath,
	}

	timerChan := make(<-chan time.Time)
	if timelimit > 0 {
		timer := time.NewTimer(time.Duration(timelimit) * time.Second)
		defer timer.Stop()
		timerChan = timer.C
	}

	defer close(pathChan)
	for i := 0; requests == 0 || i < requests; i++ {
		select {
		case <-cancelChan:
			return
		case <-timerChan:
			return
		case pathChan <- msg:
			stats.total++
		}
	}
}

func readUrlsFromFile(pathChan chan Msg, cancelChan chan struct{}, stats *stats) {
	Addrs := []Msg{}
	f, err := os.Open(urlListFilePath)
	if err != nil {
		log.Fatalf("Failed to read file %s: %s\n", urlListFilePath, err)
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
		Addrs = append(Addrs, Msg{
			method: strings.ToUpper(raw[0]),
			url:    strings.Join(raw[1:], ""),
		})
	}

	timerChan := make(<-chan time.Time)
	if timelimit > 0 {
		timer := time.NewTimer(time.Duration(timelimit) * time.Second)
		defer timer.Stop()
		timerChan = timer.C
	}

	defer close(pathChan)
	for i := 0; requests == 0 || i < requests; i++ {
		select {
		case <-cancelChan:
			return
		case <-timerChan:
			return
		case pathChan <- Addrs[i%len(Addrs)]:
			stats.total++
		}
	}

}
