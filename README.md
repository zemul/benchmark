# benchmark
benchmark支持并发访问HTTP服务，可自定义body大小及content-type，方便测试一些apache bench无法覆盖的场景

- filepath[default=/tmp/benchmark.txt] 指定数据集文件路径
- contentType 指定contentType, 可选[text/plain, application/json, multipart/form-data]
- worker 指定并发读
- maxByte body最大字节数
- minByte body最小字节数

- write[default=true],开启写
- read[default=false]，开启读
- delete[default=true],测试完成后是否删除测试数据(需要远程服务支持RESTful规范)

生成数据集相关参数：
- genPath 指定需要生成的完整http Path
- genNum 指定需要生成的数量
- genParam 指定自定义参数，追加在路径参数中 [如-genParam replication=000,生成的数据集则是 schema:domain/path?replication=000]