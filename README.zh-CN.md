# StdGee

> StdGee 保留了 Gee 的学习友好 API，但把路由能力交还给现代 `net/http`。

[English README](README.md)

StdGee 是一个教学型小项目：它把 Gee-Web 的核心体验重建在 Go 标准库之上。它依然保留了 `Engine`、`Group`、`Context`、模板、静态文件、日志/恢复中间件和优雅关闭这些适合学习的入口，但路由匹配、方法匹配、路径通配符和 HTTP 语义则尽量交给 `http.ServeMux` 负责。

## 项目速览

- 面向读者：已经学过 Gee-Web，想看看 Go 1.22 之后同类框架应该怎么写的人。
- 版本基线：当前仓库使用 `go 1.25.0`。
- 核心结论：Go 1.22 之后，很多“小框架底层能力”已经不值得再手写一遍。
- 可选扩展：仓库还演示了 Go 1.25 的 `http.CrossOriginProtection` 如何接入中间件链。

## 快速开始

```bash
go run .
```

启动后访问 `http://localhost:9999`，可以直接尝试：

- `GET /`
- `GET /ping`
- `GET /posts/`
- `GET /v1/hello/gopher`
- `GET /v1/json`
- `GET /assets/hello.txt`
- `GET /v1/panic`

表单示例可以这样测：

```bash
curl -X POST -d "username=gopher&password=123456" http://localhost:9999/v1/login
```

## 为什么 Go 1.22 之后这件事更值得做

Go 1.22 大幅增强了 `http.ServeMux`，这改变了小型 Web 框架和教学项目的设计取舍。

`ServeMux` 现在原生支持：

- 带方法的模式串，例如 `GET /posts/{id}`
- 单段通配符，例如 `{id}`
- 剩余路径通配符，例如 `{filepath...}`
- 精确尾斜杠匹配，例如 `{$}`
- 通过 `req.PathValue("id")` 读取路径参数
- 为 `GET` 自动兼容 `HEAD`
- 在方法不匹配时自动返回 `405 Method Not Allowed`，并设置 `Allow`

这意味着像 StdGee 这样的项目，不再需要把大量复杂度花在：

- 自己实现 trie 路由树
- 维护一套独立的路径参数表
- 手写方法分发逻辑
- 手动补齐 `HEAD` 和 `405` 语义

这个仓库真正想教给读者的，就是一句话：保留友好的 API，把已经不值得自己维护的路由底层删掉。

## Gee-Web 与 StdGee 的区别

| 维度 | Gee-Web 风格 | StdGee |
| --- | --- | --- |
| 路由核心 | 自己实现 trie | 直接使用 `http.ServeMux` |
| 路由语法 | `:name`、`*filepath` | `{name}`、`{filepath...}`、`{$}` |
| 方法匹配 | 框架自己管理 | 标准库模式串，如 `GET /path` |
| 路径参数 | 自己维护 params map | `req.PathValue` |
| 中间件模型 | `Context.Next()` 链式调用 | `func(http.Handler) http.Handler` |
| `GET` 对 `HEAD` | 通常要自己补 | 标准库内建 |
| `405` / `Allow` | 通常要自己补 | 标准库内建 |
| 路由冲突处理 | 框架自定规则 | `ServeMux` 在注册冲突时直接 panic |
| 优雅关闭 | 教程里常被省略 | 通过 `http.Server.Shutdown` 直接支持 |

## 哪些能力来自 `net/http`，哪些还属于 StdGee

由标准库负责：

- 路由模式解析与匹配
- 路径通配符提取
- `GET` 自动兼容 `HEAD`
- `405 Method Not Allowed`
- 冲突路由注册时的 panic

由 StdGee 负责：

- `Group("/v1")` 这样的 Gee 风格分组
- 中间件链的收集与包裹顺序
- `String`、`JSON`、`HTML` 等 `Context` 响应辅助
- 模板加载与渲染入口
- 静态文件挂载辅助
- 对优雅关闭的封装

## Go 1.22 到底带来了什么新用法

### 1. 带方法的路由模式串

现在可以直接把方法和路径写在一起：

```go
r.HandleFunc("GET /ping", func(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("pong\n"))
})
```

这样就不再需要在路由器内部额外维护一套 `GET` / `POST` 分发表。

### 2. 原生路径通配符

标准库已经理解下面这些模式：

```go
GET /v1/hello/{name}
GET /assets/{filepath...}
GET /posts/{$}
```

- `{name}` 匹配单个路径段。
- `{filepath...}` 匹配剩余全部路径。
- `{$}` 只匹配当前路径本身，不匹配子路径。

### 3. 原生路径参数读取

不需要自己在框架里维护参数表，直接读：

```go
name := req.PathValue("name")
```

StdGee 的 `Context.Param` 本质上只是对这个标准库能力做了一层便捷封装。

### 4. 更完整的默认 HTTP 语义

如果你注册了 `GET /ping`，标准库会自动处理 `HEAD /ping`。
如果路径匹配上了但方法不匹配，`ServeMux` 会自动返回 `405 Method Not Allowed`，并带上 `Allow` 头。

这正是 Go 1.22 之后最值得删除的那部分“自研框架代码”。

## 两种注册风格

StdGee 有意同时支持“标准库优先”和“Gee 风格糖衣”两种写法。

### 标准库优先写法

```go
v1 := r.Group("/v1")

v1.HandleFunc("GET /json", func(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"message": "hello from stdgee",
		"path":    req.URL.Path,
		"route":   req.Pattern,
	})
})
```

这种写法更直接地体现了现代 `net/http` 的使用方式。

### Gee 风格糖衣

```go
v1 := r.Group("/v1")

v1.GET("/hello/{name}", func(c *stdgee.Context) {
	c.String(http.StatusOK, "hello %s, you are at %s\n", c.Param("name"), c.Path)
})
```

这种写法保留了 Gee 的上手体验，但底层路由语义依旧来自 `ServeMux`。

## 对外学习面的核心 API

引擎相关：

- `stdgee.New()`
- `Engine.Handle`
- `Engine.HandleFunc`
- `Engine.GET`
- `Engine.POST`
- `Engine.Group`
- `Engine.Use`
- `Engine.Run`
- `Engine.Shutdown`

上下文辅助：

- `Context.Param`
- `Context.Query`
- `Context.PostForm`
- `Context.String`
- `Context.JSON`
- `Context.HTML`
- `Context.Data`

模板与静态资源：

- `SetFuncMap`
- `LoadHTMLGlob`
- `Static`

中间件签名：

```go
type Middleware func(http.Handler) http.Handler
```

这个签名和整个 Go `net/http` 生态更一致，比框架私有的 `Next()` 模型更容易复用。

## 中间件、模板、静态文件与优雅关闭

### 中间件

```go
r.Use(stdgee.Logger(), stdgee.Recovery())
```

父组中间件会被子组继承，子组中间件会包裹得更靠近 handler。仓库里的测试已经验证了最终执行顺序。

### 模板

```go
r.SetFuncMap(template.FuncMap{
	"FormatAsDateTime": func(t time.Time) string {
		return t.Format("2006-01-02 15:04:05")
	},
})
r.LoadHTMLGlob("templates/*")
```

真正负责渲染的依然是 `html/template`，StdGee 只是给它提供了一个方便的挂载点。

### 静态文件

```go
r.Static("/assets", "./static")
```

底层仍然是标准库的 `http.FileServer` 和 `http.StripPrefix`。

### 优雅关闭

```go
serverErr := make(chan error, 1)
go func() {
	serverErr <- r.Run(":9999")
}()

shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

_ = r.Shutdown(shutdownCtx)
```

和很多只讲“跑起来”的教程不同，StdGee 显式持有 `http.Server`，所以优雅关闭本身就是公开能力的一部分。

## 从 Gee-Web 迁移到 StdGee

### 1. 先改路由语法

- `:name` 改成 `{name}`
- `*filepath` 改成 `{filepath...}`

示例：

```go
// Gee-Web
v1.GET("/hello/:name", handler)

// StdGee
v1.GET("/hello/{name}", handler)
```

### 2. 再改路径参数读取方式

如果你继续使用 StdGee 的 `Context`，依然可以写：

```go
c.Param("name")
```

如果你直接使用标准库 handler，就写：

```go
req.PathValue("name")
```

### 3. 把中间件迁移到标准库签名

Gee-Web 的中间件通常围绕框架上下文和 `Next()` 展开。
StdGee 的中间件签名是：

```go
func(http.Handler) http.Handler
```

这样更容易复用整个 Go 生态里现成的 `net/http` 中间件。

## 可选的 Go 1.25 扩展：`CrossOriginProtection`

因为当前仓库基线是 `go 1.25.0`，你还可以接入标准库新增的跨源保护：

```go
cop := http.NewCrossOriginProtection()
cop.AddTrustedOrigin("https://example.com")

r.Use(stdgee.ProtectCrossOrigin(cop))
```

这个能力是可选的。StdGee 的核心教学重点依然是 Go 1.22 带来的现代路由模型。

## 项目结构

```text
Gee-Web-Std/
|-- go.mod
|-- main.go
|-- README.md
|-- README.zh-CN.md
|-- static/
|   `-- hello.txt
|-- templates/
|   `-- index.html
`-- stdgee/
    |-- context.go
    |-- engine.go
    |-- engine_test.go
    `-- middleware.go
```

## 延伸阅读

- Go 1.22 Release Notes: https://go.dev/doc/go1.22
- Go 1.22 Release Blog: https://go.dev/blog/go1.22
- Go 1.25 Release Notes: https://go.dev/doc/go1.25
- Go 1.25 对应的 `net/http` 文档: https://pkg.go.dev/net/http@go1.25.0

## 一句话总结

Gee-Web 教你“框架底层是怎么自己造出来的”；StdGee 教你“到了现代 Go，这些底层里哪些已经应该交回标准库”。
