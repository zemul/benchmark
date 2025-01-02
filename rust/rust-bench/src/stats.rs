use std::collections::HashMap;
use std::sync::Mutex;
use std::time::{Duration, Instant};

const BENCH_RESOLUTION: usize = 10000; // 0.1 microsecond
const BENCH_BUCKET: u128 = 1_000_000_000 / BENCH_RESOLUTION as u128;
const PERCENTILES: &[usize] = &[50, 66, 75, 80, 90, 95, 98, 99, 100];
const METHODS: &[&str] = &["HEAD", "GET", "POST", "PUT", "DELETE"];

#[derive(Default, Clone)]
pub struct LocalStat {
    pub completed: u64,
    pub failed: u32,
    pub not_2xx: u32,
    pub total: u32,
    pub transferred: u64,
    pub req_transfer: u64,
    pub resp_transfer: u64,
}

pub struct Stats {
    data: Mutex<HashMap<String, Vec<u32>>>,        // 响应时间分布
    overflow: Mutex<HashMap<String, Vec<u32>>>,    // 超出范围的响应时间
    local_stats: Mutex<HashMap<String, Vec<LocalStat>>>, // 每个工作线程的统计
    pub start: Mutex<Option<Instant>>,
    pub end: Mutex<Option<Instant>>,
    pub total: Mutex<u64>,
    workers: usize,
}

impl Stats {
    pub fn new(workers: usize) -> Self {
        let mut data = HashMap::new();
        let mut overflow = HashMap::new();
        let mut local_stats = HashMap::new();

        for &method in METHODS {
            data.insert(method.to_string(), vec![0; BENCH_RESOLUTION]);
            overflow.insert(method.to_string(), Vec::new());
            local_stats.insert(method.to_string(), vec![LocalStat::default(); workers]);
        }

        Self {
            data: Mutex::new(data),
            overflow: Mutex::new(overflow),
            local_stats: Mutex::new(local_stats),
            start: Mutex::new(Some(Instant::now())),
            end: Mutex::new(None),
            total: Mutex::new(0),
            workers,
        }
    }

    pub fn add_sample(&self, method: &str, _idx: usize, duration: Duration) {
        let index = duration.as_nanos() / BENCH_BUCKET;
        let mut data = self.data.lock().unwrap();
        let mut overflow = self.overflow.lock().unwrap();

        if index < BENCH_RESOLUTION as u128 {
            data.get_mut(method).unwrap()[index as usize] += 1;
        } else {
            overflow.get_mut(method).unwrap().push(index as u32);
        }
    }

    pub fn update_stats(&self, method: &str, worker_id: usize, status: u16, length: u64) {
        let mut local_stats = self.local_stats.lock().unwrap();
        let stat = &mut local_stats.get_mut(method).unwrap()[worker_id];
        
        stat.completed += 1;
        stat.resp_transfer += length;
        
        if !(200..300).contains(&status) {
            stat.not_2xx += 1;
            stat.failed += 1;
        }
    }

    pub fn increment_failed(&self, method: &str, worker_id: usize) {
        let mut local_stats = self.local_stats.lock().unwrap();
        local_stats.get_mut(method).unwrap()[worker_id].failed += 1;
    }

    pub fn print_stats(&self) {
        println!("\n------------ Summary ----------\n");

        let mut completed = 0;
        let mut failed = 0;
        let mut not_2xx = 0;
        let mut transferred = 0;

        let local_stats = self.local_stats.lock().unwrap();
        for stats in local_stats.values() {
            for stat in stats {
                completed += stat.completed;
                failed += stat.failed;
                not_2xx += stat.not_2xx;
                transferred += stat.transferred + stat.req_transfer + stat.resp_transfer;
            }
        }

        let start = *self.start.lock().unwrap();
        let end = self.end.lock().unwrap().unwrap_or_else(Instant::now);
        let duration = end.duration_since(start.unwrap());
        let seconds = duration.as_secs_f64();

        println!("Concurrency Level:      {}", self.workers);
        println!("Time taken for tests:   {:.3} seconds", seconds);
        println!("Complete requests:      {}", completed);
        println!("Failed requests:        {}", failed);
        println!("Failed requests(not 2xx): {}", not_2xx);
        println!("Total transferred:      {} bytes", transferred);
        println!("Requests per second:    {:.2} [#/sec]", completed as f64 / seconds);
        println!("Transfer rate:          {:.2} [Kbytes/sec]", transferred as f64 / 1024.0 / seconds);
    }

    pub fn print_stats_with_method(&self, method: &str) {
        let local_stats = self.local_stats.lock().unwrap();
        if let Some(stats) = local_stats.get(method) {
            let mut completed = 0;
            let mut failed = 0;
            let mut transferred = 0;

            for stat in stats {
                completed += stat.completed;
                failed += stat.failed;
                transferred += stat.transferred + stat.req_transfer + stat.resp_transfer;
            }

            if completed == 0 {
                return;
            }

            let start = *self.start.lock().unwrap();
            let end = self.end.lock().unwrap().unwrap_or_else(Instant::now);
            let duration = end.duration_since(start.unwrap());
            let seconds = duration.as_secs_f64();

            println!("\n------------ {} ----------\n", method);
            println!("Concurrency Level:      {}", self.workers);
            println!("Time taken for tests:   {:.3} seconds", seconds);
            println!("Complete requests:      {}", completed);
            println!("Failed requests:        {}", failed);
            println!("Total transferred:      {} bytes", transferred);
            println!("Requests per second:    {:.2} [#/sec]", completed as f64 / seconds);
            println!("Transfer rate:          {:.2} [Kbytes/sec]", transferred as f64 / 1024.0 / seconds);

            self.print_percentiles(method);
        }
    }

    fn print_percentiles(&self, method: &str) {
        let data = self.data.lock().unwrap();
        let overflow = self.overflow.lock().unwrap();
        
        let mut n: u32 = 0;
        let mut sum = 0;
        let mut min = u32::MAX;
        let mut max = 0;

        // 计算基本统计信息
        for (i, &count) in data[method].iter().enumerate() {
            if count > 0 {
                n += count;
                sum += count * i as u32;
                min = min.min(i as u32);
                max = max.max(i as u32);
            }
        }

        if n == 0 {
            return;
        }

        let avg = sum as f64 / n as f64;

        // 计算标准差
        let mut variance_sum = 0.0;
        for (i, &count) in data[method].iter().enumerate() {
            if count > 0 {
                let d = i as f64 - avg;
                variance_sum += d * d * count as f64;
            }
        }
        for &value in &overflow[method] {
            let d = value as f64 - avg;
            variance_sum += d * d;
        }
        let std_dev = (variance_sum / n as f64).sqrt();

        println!("\nConnection Times (ms)");
        println!("              min      avg        max      std");
        println!("Total:        {:.1}      {:.1}       {:.1}      {:.1}",
            min as f64 / 10.0,
            avg / 10.0,
            max as f64 / 10.0,
            std_dev / 10.0
        );

        // 打印百分位数
        println!("\nPercentage of the requests served within a certain time (ms)");
        
        let mut percentiles = Vec::new();
        for &p in PERCENTILES {
            percentiles.push((n as usize * p) / 100);
        }
        let last_idx = percentiles.len() - 1;
        percentiles[last_idx] = n as usize;

        let mut current_sum: u32 = 0;
        let mut percentile_idx = 0;

        // 处理常规数据的百分位数
        for (i, &count) in data[method].iter().enumerate() {
            if count == 0 {
                continue;
            }
            current_sum += count;
            while percentile_idx < percentiles.len() && current_sum >= percentiles[percentile_idx] as u32 {
                println!("  {}%    {:.1} ms",
                    PERCENTILES[percentile_idx],
                    i as f64 / 10.0
                );
                percentile_idx += 1;
            }
        }

        // 处理溢出数据的百分位数
        let mut overflow_values = overflow[method].clone();
        overflow_values.sort_unstable();
        for value in overflow_values {
            current_sum += 1;
            while percentile_idx < percentiles.len() && current_sum >= percentiles[percentile_idx] as u32 {
                println!("  {}%    {:.1} ms",
                    PERCENTILES[percentile_idx],
                    value as f64 / 10.0
                );
                percentile_idx += 1;
            }
        }
    }

    pub fn check_progress(&self) {
        let local_stats = self.local_stats.lock().unwrap();
        let mut completed = 0;

        for stats in local_stats.values() {
            for stat in stats {
                completed += stat.completed;
            }
        }

        println!("Completed {} requests", completed);
    }
}
