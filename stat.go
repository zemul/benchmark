package main

import (
	"fmt"
	"math"
	"net/http"
	"sort"
	"sync"
	"time"
)

const (
	benchResolution = 10000 // 0.1 microsecond
	benchBucket     = 1000000000 / benchResolution
)

var Res = []string{http.MethodHead, http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete}

type stats struct {
	start time.Time
	end   time.Time
	total int

	data     map[string][]int
	overflow map[string][]int

	localStats map[string][]stat
	sync.Mutex
}

type stat struct {
	completed    int64
	failed       int
	not2xx       int
	total        int
	transferred  int64
	reqtransfer  int
	resptransfer int
}

type Msg struct {
	method string
	url    string
}

var percentages = []int{50, 66, 75, 80, 90, 95, 98, 99, 100}

func newStats(n int) *stats {
	v := &stats{
		data:       make(map[string][]int),
		overflow:   make(map[string][]int),
		localStats: make(map[string][]stat)}

	for _, res := range Res {
		v.data[res] = make([]int, benchResolution)
		v.overflow[res] = make([]int, 0)
		v.localStats[res] = make([]stat, n)
	}
	return v

}

func (s *stats) addSample(method string, idx int, d time.Duration) {
	s.Lock()
	defer s.Unlock()

	index := int(d / benchBucket) // 耗时 d/100000
	if index < 0 {
		fmt.Printf("This request takes %3.1f seconds, skipping!\n", float64(index)/10000)
	} else if index < len(s.data[method]) { // 0.1 microsecond,精确到毫秒后一位，耗时1ms=index = 1000000 / 10000=10
		s.data[method][int(d/benchBucket)]++
	} else { // >1s放这里
		s.overflow[method] = append(s.overflow[method], index)
	}
}

func (s *stats) printStatsWithMethod(method string) {

	completed, failed, transferred, total := 0, 0, int64(0), s.total
	for _, localStat := range s.localStats[method] {
		completed += int(localStat.completed)
		failed += int(localStat.failed)
		transferred += localStat.transferred
		total += localStat.total
		transferred += int64(localStat.reqtransfer)
		transferred += int64(localStat.resptransfer)
	}
	if completed == 0 {
		return
	}
	timeTaken := float64(int64(s.end.Sub(s.start))) / 1000000000
	fmt.Printf("\n------------ %s ----------\n", method)
	fmt.Printf("\nConcurrency Level:      %d\n", workerNum)
	fmt.Printf("Time taken for tests:   %.3f seconds\n", timeTaken)
	fmt.Printf("Complete requests:      %d\n", completed)
	fmt.Printf("Failed requests:        %d\n", failed)
	fmt.Printf("Total transferred:      %d bytes\n", transferred)
	fmt.Printf("Requests per second:    %.2f [#/sec]\n", float64(completed)/timeTaken)
	fmt.Printf("Transfer rate:          %.2f [Kbytes/sec]\n", float64(transferred)/1024/timeTaken)
	n, sum := 0, 0
	min, max := 10000000, 0
	// 0.1ms - 1s
	for i := 0; i < len(s.data[method]); i++ {
		n += s.data[method][i]
		sum += s.data[method][i] * i
		if s.data[method][i] > 0 {
			if min > i {
				min = i
			}
			if max < i {
				max = i
			}
		}
	}
	n += len(s.overflow[method])
	for i := 0; i < len(s.overflow[method]); i++ {
		sum += s.overflow[method][i]
		if min > s.overflow[method][i] {
			min = s.overflow[method][i]
		}
		if max < s.overflow[method][i] {
			max = s.overflow[method][i]
		}
	}
	avg := float64(sum) / float64(n)
	varianceSum := 0.0
	for i := 0; i < len(s.data[method]); i++ {
		if s.data[method][i] > 0 {
			d := float64(i) - avg
			varianceSum += d * d * float64(s.data[method][i])
		}
	}
	for i := 0; i < len(s.overflow[method]); i++ {
		d := float64(s.overflow[method][i]) - avg
		varianceSum += d * d
	}
	std := math.Sqrt(varianceSum / float64(n))
	fmt.Printf("\nConnection Times (ms)\n")
	fmt.Printf("              min      avg        max      std\n")
	fmt.Printf("Total:        %2.1f      %3.1f       %3.1f      %3.1f\n", float32(min)/10, float32(avg)/10, float32(max)/10, std/10)
	// printing percentiles
	fmt.Printf("\nPercentage of the requests served within a certain time (ms)\n")
	percentiles := make([]int, len(percentages))
	for i := 0; i < len(percentages); i++ {
		percentiles[i] = n * percentages[i] / 100
	}
	percentiles[len(percentiles)-1] = n
	percentileIndex := 0
	currentSum := 0
	for i := 0; i < len(s.data[method]); i++ {
		currentSum += s.data[method][i]
		if s.data[method][i] > 0 && percentileIndex < len(percentiles) && currentSum >= percentiles[percentileIndex] {
			fmt.Printf("  %3d%%    %5.1f ms\n", percentages[percentileIndex], float32(i)/10.0)
			percentileIndex++
			for percentileIndex < len(percentiles) && currentSum >= percentiles[percentileIndex] {
				percentileIndex++
			}
		}
	}
	sort.Ints(s.overflow[method])
	for i := 0; i < len(s.overflow[method]); i++ {
		currentSum++
		if percentileIndex < len(percentiles) && currentSum >= percentiles[percentileIndex] {
			fmt.Printf("  %3d%%    %5.1f ms\n", percentages[percentileIndex], float32(s.overflow[method][i])/10.0)
			percentileIndex++
			for percentileIndex < len(percentiles) && currentSum >= percentiles[percentileIndex] {
				percentileIndex++
			}
		}
	}
}

func (s *stats) printStats() {
	fmt.Printf("\n------------ %s ----------\n", "Summary")

	completed, failed, not2xx, transferred, total := 0, 0, 0, int64(0), s.total
	for _, localStat := range s.localStats {
		for i := range localStat {
			completed += int(localStat[i].completed)
			failed += int(localStat[i].failed)
			not2xx += int(localStat[i].not2xx)
			transferred += int64(localStat[i].reqtransfer)
			transferred += int64(localStat[i].resptransfer)
			total += int(localStat[i].total)
		}
	}
	timeTaken := float64(int64(s.end.Sub(s.start))) / 1000000000
	fmt.Printf("\nConcurrency Level:      %d\n", workerNum)
	fmt.Printf("Time taken for tests:   %.3f seconds\n", timeTaken)
	fmt.Printf("Complete requests:      %d\n", completed)
	fmt.Printf("Failed requests:        %d\n", failed)
	fmt.Printf("Failed requests(not 2xx):        %d\n", not2xx)
	fmt.Printf("Total transferred:      %d bytes\n", transferred)
	fmt.Printf("Requests per second:    %.2f [#/sec]\n", float64(completed)/timeTaken)
	fmt.Printf("Transfer rate:          %.2f [Kbytes/sec]\n", float64(transferred)/1024/timeTaken)

}

func (s *stats) checkProgress(testName string, finishChan chan bool) {
	fmt.Printf("\n------------ %s ----------\n", testName)
	ticker := time.Tick(time.Second)
	lastTime := time.Now()
	for {
		select {
		case <-finishChan:
			wait.Done()
			return
		case t := <-ticker:
			completed, transferred, _, total := 0, int64(0), t.Sub(lastTime), s.total
			for _, localStat := range s.localStats {
				for i := range localStat {
					completed += int(localStat[i].completed)
					transferred += int64(localStat[i].transferred)
					total += int(localStat[i].total)
				}
			}
			fmt.Printf("Completed %d requests\n",
				completed,
			)

			lastTime = t
		}
	}
}
