# minit-go

> **NOTE:** You don't want to use this library. You should probably be using [OpenTelemetry](https://opentelemetry.io/).

Minit is a minimal tracing library for Go. When I say minimal, I mean it: It's ~300 lines of code and has no dependencies*.

As you can expect, it doesn't have many features. For example, it only supports exporting spans to an OpenTelemetry HTTP-compatible collector, doesn't support sampling, etc. 

The bits/helpers required to inject and extract trace context in different protocols aren't implemented, either. 

Truth to be said: Because it's so tiny, it's easy to understand and modify. 

It's also easy to use in simple, non-distributed applications.

<small>*Instead of [using the OTEL Protobufs](https://gist.github.com/dgzlopes/831a393c8071193b50165df9b72d3653), we moved the important bits to Go structs (check `pkg/otel`).</small>

## Installation

```go
go get github.com/dgzlopes/minit-go
```

## Usage

This is the code:
```go
package main

import (
	"context"
	"time"

	"github.com/dgzlopes/minit"
)

type App struct {
	tracing *minit.TracingClient
}

func main() {
	tracing := minit.NewTracingClient("http://localhost:4318/v1/traces")
	defer tracing.Export()

	app := App{
		tracing: tracing,
	}

	_, ctx := tracing.StartTrace(context.Background())
	root, ctx := tracing.StartSpan(ctx, "hello!")
	root.Service.Name = "my-app"
	defer root.Finish()

	// Do something...

	app.WithChildSpan(ctx)
}

func (a *App) WithChildSpan(ctx context.Context) {
	span, ctx := a.tracing.StartSpan(ctx, "with_child_span")
	defer span.Finish()

	// Do something...

	child, _ := a.tracing.StartSpan(ctx, "child")
	defer child.Finish()

	// Do something else...

	if (true) {
		child.MarkAsFailed()
	}
}
```