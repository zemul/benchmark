use clap::Parser;
use std::sync::Arc;
use tokio;
use anyhow::Result;

mod stats;
mod worker;
mod config;
mod url_reader;

use stats::Stats;
use worker::Worker;
use config::Config;
use url_reader::UrlReader;

#[derive(Parser)]
#[command(author, version, about, long_about = None)]
struct Args {
    #[arg(short = 'c', default_value = "1")]
    workers: usize,

    #[arg(short = 'n', default_value = "0")]
    requests: usize,

    #[arg(short = 't', default_value = "0")]
    timelimit: u64,

    #[arg(short = 'k')]
    keepalive: bool,

    #[arg(short = 's', default_value = "30")]
    timeout: u64,

    #[arg(short = 'm', default_value = "GET")]
    method: String,

    #[arg(short = 'H')]
    headers: Vec<String>,

    #[arg(short = 'f')]
    url_file: Option<String>,

    #[arg(short = 'b')]
    body_file: Option<String>,

    #[arg(long = "content-type", default_value = "text/plain")]
    content_type: String,

    #[arg(long = "min", default_value = "10")]
    min_size: usize,

    #[arg(long = "max", default_value = "100")]
    max_size: usize,

    #[arg(default_value = None)]
    url: Option<String>,
}

#[tokio::main]
async fn main() -> Result<()> {
    env_logger::init();
    
    let args = Args::parse();
    let config = Config::from(args);

    // 验证参数
    validate_config(&config)?;

    println!("Running benchmark...");
    println!("Concurrency Level: {}", config.workers);
    if config.requests > 0 {
        println!("Number of requests: {}", config.requests);
    } else {
        println!("Time limit: {} seconds", config.timelimit);
    }
    println!("Target URL: {}", config.url.as_ref().unwrap_or(&"from file".to_string()));

    // 初始化统计信息
    let stats = Arc::new(Stats::new(config.workers));

    // 创建请求通道
    let (tx, _) = tokio::sync::broadcast::channel(2000);

    // 启动进度检查
    let stats_clone = stats.clone();
    let progress_handle = tokio::spawn(async move {
        let mut interval = tokio::time::interval(tokio::time::Duration::from_secs(1));
        loop {
            interval.tick().await;
            stats_clone.check_progress();
        }
    });

    // 启动URL读取任务
    let url_reader = UrlReader::new(config.clone(), tx.clone());
    tokio::spawn(url_reader.run());

    // 启动工作线程
    let mut handles = vec![];
    for id in 0..config.workers {
        let worker = Worker::new(
            id,
            tx.subscribe(),
            stats.clone(),
            config.clone(),
        );
        handles.push(tokio::spawn(worker.run()));
    }

    // 等待所有工作线程完成
    for handle in handles {
        handle.await?;
    }

    // 停止进度检查
    progress_handle.abort();

    // 设置结束时间
    *stats.end.lock().unwrap() = Some(std::time::Instant::now());

    // 打印统计信息
    println!("\nBenchmark completed!");
    stats.print_stats();
    
    // 打印每个方法的详细统计
    for method in &["HEAD", "GET", "POST", "PUT", "DELETE"] {
        stats.print_stats_with_method(method);
    }

    Ok(())
}

fn validate_config(config: &Config) -> Result<()> {
    if config.timelimit == 0 && config.requests == 0 {
        anyhow::bail!("Either timelimit (-t) or number of requests (-n) must be specified");
    }
    if config.url.is_none() && config.url_file.is_none() {
        anyhow::bail!("Either URL or URL file (-f) must be specified");
    }
    if config.url.is_some() && config.url_file.is_some() {
        anyhow::bail!("Cannot specify both URL and URL file");
    }
    Ok(())
}
