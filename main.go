package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/bradfitz/gomemcache/memcache"
	"go.opentelemetry.io/contrib/instrumentation/github.com/bradfitz/gomemcache/memcache/otelmemcache"
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
	sess := initSession()
	mux.Handle("/fib", otelhttp.NewHandler(http.HandlerFunc(fb.serveFib), "fibEndpoint"))
	mux.Handle("/get", otelhttp.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d, err := sess.Get(r.Context(), r.URL.Query().Get("key"))
		if err != nil {
			span := trace.SpanFromContext(r.Context())
			defer span.End(trace.WithRecord())
			fmt.Fprintln(w, err.Error())
			return
		}
		fmt.Println(d)
	}), "getData"))
	mux.Handle("/write", otelhttp.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info, err := ioutil.ReadAll(r.Body)
		if err != nil {
			span := trace.SpanFromContext(r.Context())
			defer span.End(trace.WithRecord())
			fmt.Fprintln(w, err.Error())
			return
		}
		k := r.URL.Query().Get("key")
		err = sess.Save(r.Context(), k, info)
		if err != nil {
			span := trace.SpanFromContext(r.Context())
			span.SetAttributes(label.String("unrecordedValue", k))
			defer span.End(trace.WithRecord())
			fmt.Fprintln(w, err.Error())
			return
		}
		fmt.Fprintln(w, len(info), "bytes written")
	}), "writeData"))
	log.Println("Listening at address", addr)
	http.ListenAndServe(addr, mux)
}

func initSession() Session {
	return NewSession("localhost:11211", "sessionService")
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

type cache struct {
	mem *otelmemcache.Client
}

type Session interface {
	Save(ctx context.Context, k string, v []byte) error
	Get(ctx context.Context, k string) ([]byte, error)
	Drop(ctx context.Context, k string) error
}

func NewSession(addr string, service string) *cache {
	cl := memcache.New(addr)
	ct := otelmemcache.NewClientWithTracing(cl, otelmemcache.WithServiceName(service))
	return &cache{mem: ct}
}

func (c *cache) Save(ctx context.Context, k string, v []byte) error {
	item := memcache.Item{Key: k, Value: v}
	err := c.mem.Add(&item)
	if err != nil {
		span := trace.SpanFromContext(ctx)
		defer span.End(trace.WithRecord())
		span.RecordError(err)
		return err
	}
	return nil
}

func (c *cache) Get(ctx context.Context, k string) ([]byte, error) {
	i, err := c.mem.Get(k)
	if err != nil {
		span := trace.SpanFromContext(ctx)
		defer span.End(trace.WithRecord())
		span.RecordError(err)
		return nil, err
	}
	return i.Value, nil
}

func (c *cache) Drop(ctx context.Context, k string) error {
	err := c.mem.Delete(k)
	if err != nil {
		span := trace.SpanFromContext(ctx)
		defer span.End(trace.WithRecord())
		span.SetAttributes(label.String("undeletedKey", k))
		span.RecordError(err)
		return err
	}
	return nil
}
