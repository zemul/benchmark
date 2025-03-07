# Benchmark 工具

基于seaweedfs benchmark优化的高性能的 HTTP 基准测试工具，用于模拟并发请求并测试目标服务的性能。支持多种 HTTP 方法、自定义请求头、随机生成请求体等功能，适用于对 Web 服务、API 接口等进行压力测试和性能分析。

特别适用场景
1. 同时调用多个接口，多个方法混合测试
2. 多线程测试,注意调整-cpu参数,对于nginx等高并发服务框架，测试性能较ab有明显提升
---

## 功能特性
- **并发测试**：基于fasthttp client，高效内存复用，模拟高并发场景。
- **灵活的请求配置**：
    - 支持 `GET`、`POST`、`PUT` 等 HTTP 方法。
    - 可自定义请求头（Headers）。
    - 支持随机生成请求体（Body），或从文件中读取请求体。
- **超时控制**：支持设置请求超时时间。
- **Keep-Alive**：支持长连接模式，减少连接建立的开销。
- **数据集支持**：可以从文件中读取 URL 列表，用于批量测试。
- **性能统计**：支持统计请求的响应时间、成功率等指标。
---

## 参数说明

| 参数               | 缩写 | 默认值 | 描述                                                                 |
|--------------------|------|--------|--------------------------------------------------------------------|
| `-cpu`             | -    | 1      | 使用的 CPU 核心数（IO 密集型任务建议设置为 1，避免线程切换开销）。       |
| `-c`               | -    | 1      | 并发 Worker 数量，即并发请求数。                                      |
| `-n`               | -    | 0      | 总请求数。如果为 0，则根据 `-t` 参数的时间限制运行。                   |
| `-t`               | -    | 0      | 测试的最大运行时间（秒）。如果为 0，则根据 `-n` 参数的总请求数运行。     |
| `-k`               | -    | false  | 是否启用 Keep-Alive 长连接模式。                                      |
| `-s`               | -    | 30     | 每个请求的最大超时时间（秒）。                                        |
| `-m`               | -    | GET    | HTTP 请求方法，支持 `GET`、`POST`、`PUT` 等。                         |
| `-H`               | -    | -      | 自定义请求头，格式为 `Header: Value`，例如 `Accept-Encoding: gzip`。   |
| `-min`             | -    | 10     | 随机生成请求体的最小长度（字节）。                                    |
| `-max`             | -    | 100    | 随机生成请求体的最大长度（字节）。                                    |
| `-f`               | -    | -      | 数据集文件路径，文件中每行包含一个 URL，用于批量测试。                 |
| `-b`               | -    | -      | 请求体文件路径，文件内容将作为请求体发送。                            |
| `-content-type`    | -    | -      | 请求体的 Content-Type，例如 `application/x-www-form-urlencoded`。      |

---

## 使用示例

### 1. 基于调用次数用法
对目标 URL 进行并发测试1000次：
```bash
./benchmark -c 10 -n 1000  http://example.com
```
### 2. 基本调用时间用法
对目标 URL 进行并发测试10秒：
```bash
./benchmark -c 10 -t 10  http://example.com
```

### 2.使用 Keep-Alive
对目标 URL 进行并发测试：
```bash
./benchmark -c 10 -n 1000 -k http://example.com
```

### 3.自定义请求头和请求体
对目标 URL 进行并发测试：
```bash
./benchmark -c 10 -n 1000 -m POST -H "Content-Type: application/json" -b body.json http://example.com/api
```

### 4.使用数据集文件
从文件中读取 URL 列表进行测试,累计执行1000次：
```bash
./benchmark -c 10 -n 1000 -f urls.txt
```
urls.txt
```
GET,http://example.com
POST,http://example.com
```
### 5.结果统计
```bash
Completed 899 of 10000 requests, 9.0% 898.8/s 0.5MB/s
Completed 1750 of 10000 requests, 17.5% 851.1/s 0.5MB/s
Completed 2673 of 10000 requests, 26.7% 923.0/s 0.5MB/s
Completed 3547 of 10000 requests, 35.5% 874.1/s 0.5MB/s
Completed 4426 of 10000 requests, 44.3% 878.6/s 0.5MB/s
Completed 5390 of 10000 requests, 53.9% 963.9/s 0.5MB/s
Completed 6328 of 10000 requests, 63.3% 937.6/s 0.5MB/s
Completed 7248 of 10000 requests, 72.5% 920.9/s 0.5MB/s
Completed 8188 of 10000 requests, 81.9% 939.2/s 0.5MB/s
Completed 9176 of 10000 requests, 91.8% 988.7/s 0.5MB/s
Concurrency Level:      10
Time taken for tests:   10.861 seconds
Complete requests:      10000
Failed requests:        0
Total transferred:      5485680 bytes
Requests per second:    920.69 [#/sec]
Transfer rate:          493.22 [Kbytes/sec]
Connection Times (ms)
              min      avg        max      std
Total:        8.2      10.8       73.1      3.4
Percentage of the requests served within a certain time (ms)
   50%     10.2 ms
   66%     10.5 ms
   75%     10.8 ms
   80%     11.0 ms
   90%     11.8 ms
   95%     13.4 ms
   98%     19.5 ms
   99%     26.9 ms
  100%     73.1 ms
```