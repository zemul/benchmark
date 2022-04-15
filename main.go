package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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

var path string
var param string
var files int

var (
	wait       sync.WaitGroup
	writeStats *stats
	readStats  *stats
	delStats   *stats
	Addrs      []string
)

func main() {
	log.SetFlags(log.Llongfile | log.Lmicroseconds | log.Ldate)
	flag.IntVar(&workerNum, "c", 1, "concurrent worker")
	flag.IntVar(&fileSizeMin, "minByte", 10, "body minlength (byte)")
	flag.IntVar(&fileSizeMax, "maxByte", 100, "body maxlength (byte)")
	flag.BoolVar(&write, "write", true, "enable write")
	flag.BoolVar(&read, "read", false, "enable read")
	flag.BoolVar(&delete, "delete", true, "enable delete")
	flag.StringVar(&urlListFilePath, "filepath", os.TempDir()+"/benchmark_list.txt", "filePath: dataset filePath")
	flag.StringVar(&contentType, "contentType", "multipart/form-data", "Http call contentType, options[text/plain, application/json, multipart/form-data]")
	flag.StringVar(&path, "genPath", "", "Generating HTTP addresses")
	flag.IntVar(&files, "genNum", 0, "Generating num")
	flag.StringVar(&param, "genParam", "", "replication=000")
	flag.Parse()
	if files > 0 && path != "" {
		GenFile(files)
		return
	}

	if write {
		benchWrite()
	}
	if read {
		benchRead()
	}
	if delete {
		benchDelete()
	}
}

func benchDelete() {
	fmt.Printf("\n------------ %s ----------\n", "Benchmark is finished，Delete Benchmark data start")
	finishChan := make(chan bool)
	pathChan := make(chan string)
	delStats = newStats(workerNum)
	go ReadFileIds(urlListFilePath, pathChan, delStats)
	for i := 0; i < workerNum; i++ {
		wait.Add(1)
		go deleteFromRemote(pathChan)
	}
	delStats.start = time.Now()
	go delStats.checkProgress("Delete Benchmark", finishChan)
	wait.Wait()
	wait.Add(1)
	finishChan <- true
	wait.Wait()
	close(finishChan)

}

func benchRead() {
	finishChan := make(chan bool)
	pathChan := make(chan string)
	readStats = newStats(workerNum)
	go ReadFileIds(urlListFilePath, pathChan, readStats)
	for i := 0; i < workerNum; i++ {
		wait.Add(1)
		go ReadFromRemote(pathChan, &readStats.localStats[i])
	}
	readStats.start = time.Now()
	go readStats.checkProgress("Reading Benchmark", finishChan)
	wait.Wait()
	wait.Add(1)
	finishChan <- true
	wait.Wait()
	close(finishChan)
	readStats.end = time.Now()
	readStats.printStats()
}

func benchWrite() {
	finishChan := make(chan bool)
	pathChan := make(chan string)
	writeStats = newStats(workerNum)
	go ReadFileIds(urlListFilePath, pathChan, writeStats)
	time.Sleep(1 * time.Second)
	for i := 0; i < workerNum; i++ {
		wait.Add(1)
		go WriteToRemote(pathChan, &writeStats.localStats[i])
	}
	writeStats.start = time.Now()
	go writeStats.checkProgress("Writing Benchmark", finishChan)
	wait.Wait()
	writeStats.end = time.Now()
	wait.Add(1)
	finishChan <- true
	wait.Wait()
	close(finishChan)
	writeStats.printStats()
}

func deleteFromRemote(pathChan chan string) {
	defer wait.Done()
	for addr := range pathChan {
		err := Delete(addr)
		if err == nil {
			fmt.Sprintf("success delete path:%s\n", addr)
			continue
		} else {
			log.Printf("Failed to delete %s error:%v\n", addr, err)
		}
	}
}

func ReadFromRemote(pathChan chan string, s *stat) {
	defer wait.Done()
	for addr := range pathChan {
		start := time.Now()
		_, size, _, err := Get(addr)
		if err == nil {
			s.completed++
			s.transferred += size
			readStats.addSample(time.Now().Sub(start))
		} else {
			s.failed++
			log.Printf("Failed to read %s error:%v\n", addr, err)
		}
	}
}

func WriteToRemote(pathChan chan string, s *stat) {
	defer wait.Done()
	random := rand.New(rand.NewSource(time.Now().UnixNano()))
	for row := range pathChan {
		start := time.Now()
		size := int64(fileSizeMin + random.Intn(fileSizeMax-fileSizeMin))
		reader := &FakeReader{id: uint64(rand.Uint64()), size: size, random: random}
		if err := Upload(reader, &UploadOption{
			Method:    http.MethodPost,
			UploadUrl: row,
			Filename:  filepath.Base(row),
			MimeType:  contentType,
		}); err == nil {
			s.completed++
			s.transferred += size
		} else {
			s.failed++
		}
		writeStats.addSample(time.Now().Sub(start))

	}
}

func ReadFileIds(urlListFilePath string, pathChan chan string, stats *stats) {
	Addrs = []string{}
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
		Addrs = append(Addrs, string(line))
		stats.total += 1
	}
	for i := range Addrs {
		pathChan <- Addrs[i]
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
