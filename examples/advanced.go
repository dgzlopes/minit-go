package main

import (
	"context"
	"time"

	"github.com/dgzlopes/minit"
)

func NewDBSpan(trace *minit.Trace, operation string) *minit.Span {
	span := trace.StartSpan(operation)
	span.Service.Name = "db"
	span.Service.Attributes = map[string]string{
		"db.type": "mysql",
	}
	return span
}

type App struct {
	tracing *minit.TracingClient
}

func main() {
	tracing := minit.NewTracingClient("http://localhost:4318/v1/traces")

	app := App{
		tracing: tracing,
	}

	trace := tracing.StartTrace()
	root := trace.StartSpan("main")

	// Basics
	app.RunNonTracedProcess()
	app.RunCallToDB(trace)

	// Context
	traced_ctx := trace.InjectInContext(context.Background())
	app.CallWithContext(traced_ctx)
	app.TriggerFailure(traced_ctx)
	app.WithChildSpan(traced_ctx)

	root.Finish()

	err := tracing.Export()
	if err != nil {
		panic(err)
	}
}

func (_ *App) RunNonTracedProcess() {
	time.Sleep(2 * time.Second)
}

func (_ *App) RunCallToDB(trace *minit.Trace) {
	span := NewDBSpan(trace, "query")
	span.Attributes["db.statement"] = "SELECT * FROM users"
	defer span.Finish()

	time.Sleep(1 * time.Second)

	span.Events = append(span.Events, minit.Event{
		Timestamp: time.Now(),
		Fields: map[string]string{
			"event": "query_finished",
		},
	})
}

func (a *App) CallWithContext(ctx context.Context) {
	span, _ := a.tracing.StartSpanFromCtx(ctx, "call_with_context")
	defer span.Finish()

	time.Sleep(1 * time.Second)
}

func (a *App) TriggerFailure(ctx context.Context) {
	span, _ := a.tracing.StartSpanFromCtx(ctx, "trigger_failure")
	defer span.Finish()

	time.Sleep(1 * time.Second)

	span.MarkAsFailed()
	span.Events = append(span.Events, minit.Event{
		Timestamp: time.Now(),
		Fields: map[string]string{
			"level":   "error",
			"message": "something went wrong",
		},
	})
}

func (a *App) WithChildSpan(ctx context.Context) {
	span, ctx := a.tracing.StartSpanFromCtx(ctx, "with_child_span")
	defer span.Finish()

	time.Sleep(1 * time.Second)

	child, _ := a.tracing.StartSpanFromCtx(ctx, "child")
	defer child.Finish()

	time.Sleep(1 * time.Second)
}
