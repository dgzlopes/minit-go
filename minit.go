package minit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/dgzlopes/minit/pkg/otel"
)

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

	for service_name, spans := range spansByService {
		tracingBatch := otel.TracingBatch{
			ResourceSpans: []otel.ResourceSpans{
				{
					Resource: otel.Resource{
						Attributes: []otel.Attribute{
							otel.NewAttribute("service.name", service_name),
						},
					},
					ScopeSpans: []otel.ScopeSpans{
						{
							Scope: otel.Scope{
								Name: "minit-go",
							},
						},
					},
				},
			},
		}
		tracingSpans := []otel.Span{}
		for _, span := range spans {
			tracingSpan := otel.Span{
				TraceID:           span.TraceID,
				SpanID:            span.SpanID,
				ParentSpanID:      span.ParentID,
				Name:              span.Operation,
				StartTimeUnixNano: fmt.Sprintf("%d", span.StartTime.UnixNano()),
				EndTimeUnixNano:   fmt.Sprintf("%d", span.EndTime.UnixNano()),
				Attributes:        []otel.Attribute{},
				Events:            []otel.Event{},
			}
			for k, v := range span.Attributes {
				tracingSpan.Attributes = append(tracingSpan.Attributes, otel.NewAttribute(k, v))
			}
			for _, event := range span.Events {
				tracingEvent := otel.Event{
					TimeUnixNano: fmt.Sprintf("%d", event.Timestamp.UnixNano()),
					Attributes:   []otel.Attribute{},
				}

				for k, v := range event.Fields {
					tracingEvent.Attributes = append(tracingEvent.Attributes, otel.NewAttribute(k, v))
				}
			}

			if span.IsOK {
				tracingSpan.Status.Code = otel.STATUS_CODE_OK
			} else {
				tracingSpan.Status.Code = otel.STATUS_CODE_ERROR
			}

			tracingSpans = append(tracingSpans, tracingSpan)
		}
		tracingBatch.ResourceSpans[0].ScopeSpans[0].Spans = tracingSpans

		payload, err := json.Marshal(tracingBatch)
		if err != nil {
			return fmt.Errorf("failed to marshal traceData: %w", err)
		}

		req, err := http.NewRequest("POST", t.Endpoint, bytes.NewBuffer(payload))
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

type Trace struct {
	TraceID string

	spans []*Span
	mx    sync.Mutex
}

func (tc *TracingClient) StartTrace(ctx context.Context) (*Trace, context.Context) {
	tc.mx.Lock()
	defer tc.mx.Unlock()
	trace := &Trace{
		TraceID: otel.NewTraceID(),
		spans:   []*Span{},
	}
	tc.traces = append(tc.traces, trace)
	return trace, trace.injectInCtx(ctx)
}

func (tc *TracingClient) getTraceFromCtx(ctx context.Context) *Trace {
	return ctx.Value("trace").(*Trace)
}

func (t *Trace) injectInCtx(ctx context.Context) context.Context {
	return context.WithValue(ctx, "trace", t)
}

type Span struct {
	Operation  string
	Service    Service
	Events     []Event
	Attributes map[string]string
	IsOK       bool

	TraceID  string
	SpanID   string
	ParentID string

	StartTime time.Time
	EndTime   time.Time
}

type Event struct {
	Timestamp time.Time
	Fields    map[string]string
}

type Service struct {
	Name       string
	Attributes map[string]string
}

func (tc *TracingClient) StartSpan(ctx context.Context, operation string) (*Span, context.Context) {
	trace := tc.getTraceFromCtx(ctx)
	trace.mx.Lock()
	defer trace.mx.Unlock()
	span := &Span{
		TraceID: trace.TraceID,
		SpanID:  otel.NewSpanID(),
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
	trace.spans = append(trace.spans, span)
	if ctx.Value("span") != nil {
		span.ParentID = ctx.Value("span").(*Span).SpanID
	}
	return span, context.WithValue(ctx, "span", span)
}

func (s *Span) Finish() *Span {
	s.EndTime = time.Now()
	return s
}

func (s *Span) MarkAsFailed() *Span {
	s.IsOK = false
	return s
}
