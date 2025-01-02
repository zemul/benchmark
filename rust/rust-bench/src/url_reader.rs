use crate::{Config, worker::Request};
use anyhow::Result;
use std::io::{BufRead, BufReader};
use tokio::sync::broadcast::Sender;
use tokio::time::{self, Duration};

pub struct UrlReader {
    config: Config,
    tx: Sender<Request>,
}

impl UrlReader {
    pub fn new(config: Config, tx: Sender<Request>) -> Self {
        Self { config, tx }
    }

    pub async fn run(self) -> Result<()> {
        if let Some(url) = &self.config.url {
            self.read_single_url(url).await?;
        } else if let Some(file_path) = &self.config.url_file {
            self.read_urls_from_file(file_path).await?;
        }
        Ok(())
    }

    async fn read_single_url(&self, url: &str) -> Result<()> {
        let req = Request {
            method: self.config.method.clone(),
            url: url.to_string(),
        };

        if self.config.requests > 0 {
            println!("Sending {} requests...", self.config.requests);
            for i in 0..self.config.requests {
                if i % 1000 == 0 {
                    println!("Progress: {}/{}", i, self.config.requests);
                }
                self.tx.send(req.clone())?;
            }
        } else if self.config.timelimit > 0 {
            println!("Running for {} seconds...", self.config.timelimit);
            let duration = Duration::from_secs(self.config.timelimit);
            let mut interval = time::interval(Duration::from_millis(1));
            let start = time::Instant::now();

            while start.elapsed() < duration {
                interval.tick().await;
                self.tx.send(req.clone())?;
            }
        }

        // 发送结束标记
        drop(self.tx);
        Ok(())
    }

    async fn read_urls_from_file(&self, file_path: &str) -> Result<()> {
        let file = std::fs::File::open(file_path)?;
        let reader = BufReader::new(file);
        let mut urls = Vec::new();

        for line in reader.lines() {
            let line = line?;
            let parts: Vec<&str> = line.split(',').collect();
            if parts.len() >= 2 {
                urls.push(Request {
                    method: parts[0].trim().to_uppercase(),
                    url: parts[1..].join(",").trim().to_string(),
                });
            }
        }

        if self.config.requests > 0 {
            for _ in 0..self.config.requests {
                for url in &urls {
                    self.tx.send(url.clone())?;
                }
            }
        } else if self.config.timelimit > 0 {
            let duration = Duration::from_secs(self.config.timelimit);
            let mut interval = time::interval(Duration::from_millis(1));
            let start = time::Instant::now();
            let mut index = 0;

            while start.elapsed() < duration {
                interval.tick().await;
                if index >= urls.len() {
                    index = 0;
                }
                self.tx.send(urls[index].clone())?;
                index += 1;
            }
        }

        Ok(())
    }
}
