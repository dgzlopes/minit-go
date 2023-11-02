package minit

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
)

func NewTraceID() [16]byte {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return b
}

func NewSpanID() [8]byte {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return b
}

type TracingClient struct {
	Endpoint string
	traces   []*Trace
	mx       sync.Mutex
}

func NewTracingClient(endpoint string) *TracingClient {
	return &TracingClient{
		Endpoint: endpoint,
		traces:   []*Trace{},
	}
}

func (t *TracingClient) Export() error {
	t.mx.Lock()
	defer t.mx.Unlock()

	spansByService := map[string][]*Span{}
	for _, trace := range t.traces {
		trace.mx.Lock()
		defer trace.mx.Unlock()
		for _, span := range trace.spans {
			spansByService[span.Service.Name] = append(spansByService[span.Service.Name], span)
		}
	}

	for _, spans := range spansByService {
		traceData := ptrace.NewTraces()
		for _, span := range spans {
			td := ptrace.NewTraces()

			resourceSpans := td.ResourceSpans()
			resourceSpans.EnsureCapacity(1)
			rspan := resourceSpans.AppendEmpty()
			rspan.Resource().Attributes().PutStr("service.name", span.Service.Name)
			for k, v := range span.Service.Attributes {
				rspan.Resource().Attributes().PutStr(k, v)
			}

			ilss := rspan.ScopeSpans()
			ilss.EnsureCapacity(1)
			ils := ilss.AppendEmpty()
			ils.Scope().SetName("minit-go")

			sps := ils.Spans()
			sps.EnsureCapacity(len(spans))

			for _, span := range spans {
				sp := sps.AppendEmpty()
				sp.SetTraceID(span.TraceID)
				sp.SetSpanID(NewSpanID())
				if span.ParentID != [8]byte{} {
					sp.SetParentSpanID(span.ParentID)
				}
				sp.SetName(span.Operation)
				if span.IsOK {
					sp.Status().SetCode(ptrace.StatusCodeOk)
				} else {
					sp.Status().SetCode(ptrace.StatusCodeError)
				}
				sp.SetStartTimestamp(pcommon.NewTimestampFromTime(span.StartTime))
				sp.SetEndTimestamp(pcommon.NewTimestampFromTime(span.EndTime))
				sp.Events().EnsureCapacity(len(span.Events))

				sp.Attributes().EnsureCapacity(len(span.Attributes))
				for k, v := range span.Attributes {
					sp.Attributes().PutStr(k, v)
				}

				for _, Event := range span.Events {
					ev := sp.Events().AppendEmpty()
					ev.SetTimestamp(pcommon.NewTimestampFromTime(Event.Timestamp))
					ev.Attributes().EnsureCapacity(len(Event.Fields))
					for k, v := range Event.Fields {
						ev.Attributes().PutStr(k, v)
					}
				}
				td.CopyTo(traceData)
			}
		}
		tr := ptraceotlp.NewExportRequestFromTraces(
			traceData,
		)

		request, err := tr.MarshalJSON()
		if err != nil {
			return fmt.Errorf("failed to marshal traceData: %w", err)
		}

		req, err := http.NewRequest("POST", t.Endpoint, bytes.NewBuffer(request))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to send request: %w", err)
		}
		defer resp.Body.Close()
	}
	return nil
}

type Event struct {
	Timestamp time.Time
	Fields    map[string]string
}

type Service struct {
	Name       string
	Attributes map[string]string
}

type Span struct {
	Operation  string
	Service    Service
	Events     []Event
	Attributes map[string]string
	IsOK       bool

	TraceID  [16]byte
	SpanID   [8]byte
	ParentID [8]byte

	StartTime time.Time
	EndTime   time.Time
}

type Trace struct {
	TraceID [16]byte

	spans []*Span
	mx    sync.Mutex
}

func (t *TracingClient) StartTrace() *Trace {
	t.mx.Lock()
	defer t.mx.Unlock()
	trace := &Trace{
		TraceID: NewTraceID(),
		spans:   []*Span{},
	}
	t.traces = append(t.traces, trace)
	return trace
}

func (t *TracingClient) ContinueTraceFromContext(ctx context.Context) *Trace {
	return ctx.Value("trace").(*Trace)
}

func (t *Trace) InjectInContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, "trace", t)
}

func (t *TracingClient) StartSpanFromCtx(ctx context.Context, operation string) (*Span, context.Context) {
	trace := t.ContinueTraceFromContext(ctx)
	span := trace.StartSpan(operation)
	if ctx.Value("span") != nil {
		span.ParentID = ctx.Value("span").(*Span).SpanID
	}
	return span, context.WithValue(ctx, "span", span)
}

func (t *Trace) StartSpan(operation string) *Span {
	t.mx.Lock()
	defer t.mx.Unlock()
	span := Span{
		TraceID: t.TraceID,
		SpanID:  NewSpanID(),
		Service: Service{
			Name:       "minit-go",
			Attributes: map[string]string{},
		},
		Operation:  operation,
		StartTime:  time.Now(),
		Attributes: map[string]string{},
		Events:     []Event{},
		IsOK:       true,
	}
	t.spans = append(t.spans, &span)
	return &span
}

func (s *Span) Finish() *Span {
	s.EndTime = time.Now()
	return s
}

func (s *Span) MarkAsFailed() *Span {
	s.IsOK = false
	return s
}
