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
	root, ctx := tracing.StartSpan(ctx, "main")
	defer root.Finish()

	app.RunNonTracedProcess()
	app.TriggerFailure(ctx)
	app.WithChildSpan(ctx)
	app.RunCallToDB(ctx)
}

func (_ *App) RunNonTracedProcess() {
	time.Sleep(2 * time.Second)
}

func (a *App) TriggerFailure(ctx context.Context) {
	span, _ := a.tracing.StartSpan(ctx, "trigger_failure")
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
	span, ctx := a.tracing.StartSpan(ctx, "with_child_span")
	defer span.Finish()

	time.Sleep(1 * time.Second)

	child, _ := a.tracing.StartSpan(ctx, "child")
	defer child.Finish()

	time.Sleep(1 * time.Second)
}

func (a *App) RunCallToDB(ctx context.Context) {
	span, _ := a.NewDBSpan(ctx, "query")
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

func (a *App) NewDBSpan(ctx context.Context, operation string) (*minit.Span, context.Context) {
	span, ctx := a.tracing.StartSpan(ctx, operation)
	span.Service.Name = "db"
	span.Service.Attributes = map[string]string{
		"db.type": "mysql",
	}
	return span, ctx
}
