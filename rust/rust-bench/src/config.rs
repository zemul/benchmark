use crate::Args;

#[derive(Clone)]
pub struct Config {
    // 基础配置
    pub workers: usize,      // 并发工作线程数
    pub requests: usize,     // 总请求数
    pub timelimit: u64,      // 测试时间限制(秒)
    
    // 网络配置
    pub keepalive: bool,     // 是否启用 keep-alive
    pub timeout: u64,        // 请求超时时间(秒)
    
    // 请求配置
    pub method: String,      // HTTP 方法
    pub headers: Vec<String>, // 自定义 HTTP 头
    pub url: Option<String>, // 单个URL
    pub url_file: Option<String>, // URL列表文件
    
    // 请求体配置
    pub body_file: Option<String>, // 请求体文件
    pub content_type: String,      // Content-Type
    pub min_size: usize,    // 随机请求体最小大小
    pub max_size: usize,    // 随机请求体最大大小
}

impl From<Args> for Config {
    fn from(args: Args) -> Self {
        Self {
            workers: args.workers,
            requests: args.requests,
            timelimit: args.timelimit,
            keepalive: args.keepalive,
            timeout: args.timeout,
            method: args.method,
            headers: args.headers,
            url: args.url,
            url_file: args.url_file,
            body_file: args.body_file,
            content_type: args.content_type,
            min_size: args.min_size,
            max_size: args.max_size,
        }
    }
}

impl Config {
    // 解析 HTTP 头
    pub fn get_headers(&self) -> Vec<(String, String)> {
        self.headers
            .iter()
            .filter_map(|header| {
                let parts: Vec<&str> = header.splitn(2, ':').collect();
                if parts.len() == 2 {
                    Some((
                        parts[0].trim().to_string(),
                        parts[1].trim().to_string(),
                    ))
                } else {
                    None
                }
            })
            .collect()
    }

    // 验证配置是否有效
    pub fn validate(&self) -> anyhow::Result<()> {
        // 检查必需参数
        if self.timelimit == 0 && self.requests == 0 {
            anyhow::bail!("Either timelimit (-t) or number of requests (-n) must be specified");
        }

        // 检查 URL 配置
        if self.url.is_none() && self.url_file.is_none() {
            anyhow::bail!("Either URL or URL file (-f) must be specified");
        }
        if self.url.is_some() && self.url_file.is_some() {
            anyhow::bail!("Cannot specify both URL and URL file");
        }

        // 检查工作线程数
        if self.workers == 0 {
            anyhow::bail!("Number of workers must be greater than 0");
        }

        // 检查请求体大小范围
        if self.min_size > self.max_size {
            anyhow::bail!("Minimum size cannot be greater than maximum size");
        }

        // 检查 HTTP 方法
        match self.method.to_uppercase().as_str() {
            "GET" | "POST" | "PUT" | "DELETE" | "HEAD" => Ok(()),
            _ => anyhow::bail!("Unsupported HTTP method: {}", self.method),
        }
    }

    // 获取 Content-Type
    pub fn get_content_type(&self) -> &str {
        &self.content_type
    }

    // 检查是否使用 keep-alive
    pub fn use_keepalive(&self) -> bool {
        self.keepalive
    }

    // 获取超时时间
    pub fn get_timeout(&self) -> std::time::Duration {
        std::time::Duration::from_secs(self.timeout)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_header_parsing() {
        let config = Config {
            headers: vec![
                "Accept: application/json".to_string(),
                "User-Agent: rust-bench".to_string(),
            ],
            ..Default::default()
        };

        let headers = config.get_headers();
        assert_eq!(headers.len(), 2);
        assert_eq!(headers[0].0, "Accept");
        assert_eq!(headers[0].1, "application/json");
        assert_eq!(headers[1].0, "User-Agent");
        assert_eq!(headers[1].1, "rust-bench");
    }

    #[test]
    fn test_validation() {
        // 有效配置
        let valid_config = Config {
            workers: 1,
            requests: 100,
            timelimit: 0,
            method: "GET".to_string(),
            url: Some("http://example.com".to_string()),
            ..Default::default()
        };
        assert!(valid_config.validate().is_ok());

        // 无效配置 - 缺少请求数和时间限制
        let invalid_config = Config {
            workers: 1,
            requests: 0,
            timelimit: 0,
            method: "GET".to_string(),
            url: Some("http://example.com".to_string()),
            ..Default::default()
        };
        assert!(invalid_config.validate().is_err());
    }
}

// 为 Config 实现 Default trait
impl Default for Config {
    fn default() -> Self {
        Self {
            workers: 1,
            requests: 0,
            timelimit: 0,
            keepalive: false,
            timeout: 30,
            method: "GET".to_string(),
            headers: Vec::new(),
            url: None,
            url_file: None,
            body_file: None,
            content_type: "text/plain".to_string(),
            min_size: 10,
            max_size: 100,
        }
    }
}
