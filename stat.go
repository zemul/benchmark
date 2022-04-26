package main

import (
	"fmt"
	"math"
	"sort"
	"time"
)

const (
	benchResolution = 10000 // 0.1 microsecond
	benchBucket     = 1000000000 / benchResolution
)

type stats struct {
	data       []int
	overflow   []int
	localStats []stat
	start      time.Time
	end        time.Time
	total      int
}

type stat struct {
	completed    int
	failed       int
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
	return &stats{
		data:       make([]int, benchResolution),
		overflow:   make([]int, 0),
		localStats: make([]stat, n),
	}
}

func (s *stats) addSample(d time.Duration) {
	index := int(d / benchBucket) // 耗时 d/100000
	if index < 0 {
		fmt.Printf("This request takes %3.1f seconds, skipping!\n", float64(index)/10000)
	} else if index < len(s.data) { // 0.1 microsecond,精确到毫秒后一位，耗时1ms=index = 1000000 / 10000=10
		s.data[int(d/benchBucket)]++
	} else { // >1s放这里
		s.overflow = append(s.overflow, index)
	}
}

func (s *stats) printStats() {
	completed, failed, transferred, total := 0, 0, int64(0), s.total
	for _, localStat := range s.localStats {
		completed += localStat.completed
		failed += localStat.failed
		transferred += localStat.transferred
		total += localStat.total
	}
	timeTaken := float64(int64(s.end.Sub(s.start))) / 1000000000
	fmt.Printf("\nConcurrency Level:      %d\n", workerNum)
	fmt.Printf("Time taken for tests:   %.3f seconds\n", timeTaken)
	fmt.Printf("Complete requests:      %d\n", completed)
	fmt.Printf("Failed requests:        %d\n", failed)
	fmt.Printf("Total transferred:      %d bytes\n", transferred)
	fmt.Printf("Requests per second:    %.2f [#/sec]\n", float64(completed)/timeTaken)
	fmt.Printf("Transfer rate:          %.2f [Kbytes/sec]\n", float64(transferred)/1024/timeTaken)
	n, sum := 0, 0
	min, max := 10000000, 0
	for i := 0; i < len(s.data); i++ {
		n += s.data[i]
		sum += s.data[i] * i
		if s.data[i] > 0 {
			if min > i {
				min = i
			}
			if max < i {
				max = i
			}
		}
	}
	n += len(s.overflow)
	for i := 0; i < len(s.overflow); i++ {
		sum += s.overflow[i]
		if min > s.overflow[i] {
			min = s.overflow[i]
		}
		if max < s.overflow[i] {
			max = s.overflow[i]
		}
	}
	avg := float64(sum) / float64(n)
	varianceSum := 0.0
	for i := 0; i < len(s.data); i++ {
		if s.data[i] > 0 {
			d := float64(i) - avg
			varianceSum += d * d * float64(s.data[i])
		}
	}
	for i := 0; i < len(s.overflow); i++ {
		d := float64(s.overflow[i]) - avg
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
	for i := 0; i < len(s.data); i++ {
		currentSum += s.data[i]
		if s.data[i] > 0 && percentileIndex < len(percentiles) && currentSum >= percentiles[percentileIndex] {
			fmt.Printf("  %3d%%    %5.1f ms\n", percentages[percentileIndex], float32(i)/10.0)
			percentileIndex++
			for percentileIndex < len(percentiles) && currentSum >= percentiles[percentileIndex] {
				percentileIndex++
			}
		}
	}
	sort.Ints(s.overflow)
	for i := 0; i < len(s.overflow); i++ {
		currentSum++
		if percentileIndex < len(percentiles) && currentSum >= percentiles[percentileIndex] {
			fmt.Printf("  %3d%%    %5.1f ms\n", percentages[percentileIndex], float32(s.overflow[i])/10.0)
			percentileIndex++
			for percentileIndex < len(percentiles) && currentSum >= percentiles[percentileIndex] {
				percentileIndex++
			}
		}
	}
}

func (s *stats) checkProgress(testName string, finishChan chan bool) {
	fmt.Printf("\n------------ %s ----------\n", testName)
	ticker := time.Tick(time.Second)
	lastCompleted, lastTransferred, lastTime := 0, int64(0), time.Now()
	for {
		select {
		case <-finishChan:
			wait.Done()
			return
		case t := <-ticker:
			completed, transferred, taken, total := 0, int64(0), t.Sub(lastTime), s.total
			for _, localStat := range s.localStats {
				completed += localStat.completed
				transferred += localStat.transferred
				total += localStat.total
			}
			fmt.Printf("Completed %d of %d requests, %3.1f%% %3.1f/s %3.1fMB/s\n",
				completed, total, float64(completed)*100/float64(total),
				float64(completed-lastCompleted)*float64(int64(time.Second))/float64(int64(taken)),
				float64(transferred-lastTransferred)*float64(int64(time.Second))/float64(int64(taken))/float64(1024*1024),
			)
			lastCompleted, lastTransferred, lastTime = completed, transferred, t
		}
	}
}
