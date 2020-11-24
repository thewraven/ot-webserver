package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/otel/exporters/trace/jaeger"
	"go.opentelemetry.io/otel/label"
)

var serviceName = os.Getenv("SERVICE_NAME")
var addr = ":9090"

type Fibbonaccier interface {
	Fib(ctx context.Context, n int) int
}

type cachedFib struct {
	impl   Fibbonaccier
	values map[int]int
}

func NewCached(real Fibbonaccier) Fibbonaccier {
	return cachedFib{impl: real, values: make(map[int]int)}
}

func (c cachedFib) Fib(ctx context.Context, n int) int {
	ctx, span := trace.SpanFromContext(ctx).Tracer().Start(ctx, "cachedSpan")
	defer span.End()
	if v, ok := c.values[n]; ok {
		span.AddEvent("value cached",
			trace.WithAttributes(label.Int("f", n)))
		return v
	}
	v := c.impl.Fib(ctx, n)
	c.values[n] = v
	return v
}

type mathFib struct{}

func (mathFib) Fib(_ context.Context, n int) int {
	a, b := 0, 1
	for i := 0; i < n; i++ {
		a, b = b, a+b
	}
	return b
}

type otFib struct {
	ot Fibbonaccier
}

func (o otFib) Fib(ctx context.Context, n int) int {
	span := trace.SpanFromContext(ctx)
	_, span = span.Tracer().Start(ctx, "/fib invocation")
	defer span.End()
	span.SetAttributes(label.Int("Fib requested", n))
	return o.ot.Fib(ctx, n)
}

func (o otFib) serveFib(w http.ResponseWriter, r *http.Request) {
	span := trace.SpanFromContext(r.Context())
	defer span.End()
	n := r.URL.Query().Get("n")
	f, err := strconv.Atoi(n)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, "Input is not a number", n)
		return
	}
	out := o.Fib(r.Context(), f)
	fmt.Fprintln(w, out)
}

func main() {
	close := initTracer()
	defer close()
	mux := http.NewServeMux()
	cache := NewCached(mathFib{})
	fb := otFib{ot: cache}
	mux.Handle("/fib", otelhttp.NewHandler(http.HandlerFunc(fb.serveFib), "fibEndpoint"))
	log.Println("Listening at address", addr)
	http.ListenAndServe(addr, mux)
}

func initTracer() func() {
	flush, err := jaeger.InstallNewPipeline(
		jaeger.WithCollectorEndpoint("http://localhost:14268/api/traces"),
		jaeger.WithProcess(jaeger.Process{
			ServiceName: serviceName,
		}),
		jaeger.WithSDK(&sdktrace.Config{
			DefaultSampler: sdktrace.AlwaysSample(),
		}),
	)
	if err != nil {
		panic(err)
	}
	return flush
}
