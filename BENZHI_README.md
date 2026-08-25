基于 Go 实现的 TideShield 项目，一款面向潮汐风暴潮场景的防洪闸群与排涝泵站联控服务。

## 运行

使用 Go 1.26.2，在项目根目录启动：

```text
go run ./cmd/tideshield -addr 127.0.0.1:21226 -data ./data -web ./web
```

浏览器打开 `http://127.0.0.1:21226/operations.html`。服务会把事件日志与原子快照写入 `data` 目录。

## 构建

依赖已完整保存在 `vendor`，可关闭模块代理进行离线构建：

```text
go build -mod=vendor ./...
```

`benzhi.Dockerfile` 只执行离线生产构建，并保留容器内 Go 工具链和可覆盖的 `CMD`。
