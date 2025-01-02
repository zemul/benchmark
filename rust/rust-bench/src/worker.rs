use crate::{Config, Stats};
use anyhow::Result;
use bytes::Bytes;
use rand::{Rng, SeedableRng};
use rand::rngs::SmallRng;
use reqwest::{Client, Response};
use std::sync::Arc;
use std::time::Instant;
use tokio::sync::broadcast::Receiver;

pub struct Worker {
    id: usize,
    rx: Receiver<Request>,
    stats: Arc<Stats>,
    client: Client,
    config: Config,
}

#[derive(Clone, Debug)]
pub struct Request {
    pub method: String,
    pub url: String,
}

impl Worker {
    pub fn new(
        id: usize,
        rx: Receiver<Request>,
        stats: Arc<Stats>,
        config: Config,
    ) -> Self {
        let client = Client::builder()
            .timeout(config.get_timeout())
            .pool_idle_timeout(if config.use_keepalive() {
                Some(std::time::Duration::from_secs(90))
            } else {
                None
            })
            .build()
            .unwrap();

        Self {
            id,
            rx,
            stats,
            client,
            config,
        }
    }

    pub async fn run(mut self) -> Result<()> {
        let mut rng = SmallRng::from_entropy();

        while let Ok(req) = self.rx.recv().await {
            let start = Instant::now();
            let result = self.process_request(&req, &mut rng).await;
            
            match result {
                Ok(resp) => {
                    let length = resp.content_length().unwrap_or(0);
                    self.stats.update_stats(&req.method, self.id, resp.status().as_u16(), length);
                }
                Err(e) => {
                    println!("Request failed: {}", e);
                    self.stats.increment_failed(&req.method, self.id);
                }
            }

            self.stats.add_sample(&req.method, self.id, start.elapsed());
        }

        Ok(())
    }

    async fn process_request(&self, req: &Request, rng: &mut SmallRng) -> Result<Response> {
        match req.method.as_str() {
            "GET" => self.get(&req.url).await,
            "HEAD" => self.head(&req.url).await,
            "DELETE" => self.delete(&req.url).await,
            "POST" | "PUT" => self.upload(req, rng).await,
            _ => Err(anyhow::anyhow!("Unsupported method: {}", req.method)),
        }
    }

    async fn get(&self, url: &str) -> Result<Response> {
        let resp = self.client.get(url)
            .headers(self.get_headers())
            .send()
            .await?;
        Ok(resp)
    }

    async fn head(&self, url: &str) -> Result<Response> {
        let resp = self.client.head(url)
            .headers(self.get_headers())
            .send()
            .await?;
        Ok(resp)
    }

    async fn delete(&self, url: &str) -> Result<Response> {
        let resp = self.client.delete(url)
            .headers(self.get_headers())
            .send()
            .await?;
        Ok(resp)
    }

    async fn upload(&self, req: &Request, rng: &mut SmallRng) -> Result<Response> {
        let body = if let Some(ref body_file) = self.config.body_file {
            Bytes::from(std::fs::read(body_file)?)
        } else {
            let size = rng.gen_range(self.config.min_size..=self.config.max_size);
            let mut data = vec![0u8; size];
            rng.fill(&mut data[..]);
            Bytes::from(data)
        };

        let mut builder = self.client
            .request(reqwest::Method::from_bytes(req.method.as_bytes())?, &req.url)
            .headers(self.get_headers())
            .body(body);

        // 设置 content-type
        if !self.config.content_type.is_empty() {
            builder = builder.header(reqwest::header::CONTENT_TYPE, self.config.get_content_type());
        }

        let resp = builder.send().await?;
        Ok(resp)
    }

    fn get_headers(&self) -> reqwest::header::HeaderMap {
        let mut headers = reqwest::header::HeaderMap::new();
        for (key, value) in self.config.get_headers() {
            if let (Ok(key), Ok(value)) = (
                reqwest::header::HeaderName::from_bytes(key.as_bytes()),
                reqwest::header::HeaderValue::from_str(&value)
            ) {
                headers.insert(key, value);
            }
        }
        headers
    }
}
