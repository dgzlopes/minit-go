package otel

import (
	"crypto/rand"
	"fmt"
)

type TracingBatch struct {
	ResourceSpans []ResourceSpans `json:"resourceSpans"`
}

type ResourceSpans struct {
	Resource   Resource     `json:"resource"`
	ScopeSpans []ScopeSpans `json:"scopeSpans"`
}

type Resource struct {
	Attributes []Attribute `json:"attributes"`
}

type Attribute struct {
	Key   string `json:"key"`
	Value struct {
		StringValue string `json:"stringValue"`
	} `json:"value"`
}

type ScopeSpans struct {
	Scope Scope  `json:"scope"`
	Spans []Span `json:"spans"`
}

type Scope struct {
	Name string `json:"name"`
}

type Span struct {
	TraceID           string      `json:"traceId"`
	SpanID            string      `json:"spanId"`
	ParentSpanID      string      `json:"parentSpanId"`
	Name              string      `json:"name"`
	StartTimeUnixNano string      `json:"startTimeUnixNano"`
	EndTimeUnixNano   string      `json:"endTimeUnixNano"`
	Attributes        []Attribute `json:"attributes"`
	Events            []Event     `json:"events"`
	Status            struct {
		Code int `json:"code"`
	} `json:"status"`
}

type Event struct {
	TimeUnixNano string      `json:"timeUnixNano"`
	Attributes   []Attribute `json:"attributes"`
}

const STATUS_CODE_OK = 1
const STATUS_CODE_ERROR = 2

func NewTraceID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%x", b)
}

func NewSpanID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%x", b)
}

func NewStatus(code int) struct {
	Code int "json:\"code\""
} {
	return struct {
		Code int "json:\"code\""
	}{
		Code: code,
	}
}

func NewAttribute(key string, value string) Attribute {
	return Attribute{
		Key: key,
		Value: struct {
			StringValue string `json:"stringValue"`
		}{
			StringValue: value,
		},
	}
}
