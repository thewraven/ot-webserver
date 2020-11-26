package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/honeycombio/opentelemetry-exporter-go/honeycomb"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/propagators"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"go.opentelemetry.io/otel/label"
)

const (
	serviceName = "otelClient"
)

func main() {
	rand.Seed(time.Now().Unix())
	closeTracer := initTracer(initHoneycomb())
	defer closeTracer()
	cl := &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}
	addFibs(context.Background(), *cl)
	time.Sleep(time.Second * 3)
}

func addFibs(ctx context.Context, cl http.Client) {
	ctx, span := global.Tracer(serviceName).Start(ctx, "addFibs")
	defer span.End()
	var wg sync.WaitGroup
	var a, b int
	go func(ctx context.Context) {
		wg.Add(1)
		defer wg.Done()
		var err error
		ctx, cancel := context.WithTimeout(ctx, time.Second*1)
		defer cancel()
		a, err = getFib(ctx, cl)
		if err != nil {
			span.RecordError(ctx, err)
		}
	}(ctx)
	go func(ctx context.Context) {
		wg.Add(1)
		defer wg.Done()
		var err error
		ctx, cancel := context.WithTimeout(ctx, time.Second*1)
		defer cancel()
		b, err = getFib(ctx, cl)
		if err != nil {
			span.RecordError(ctx, err)
		}
	}(ctx)
	wg.Wait()
	span.AddEventWithTimestamp(ctx, time.Now(), "calculation is over,", label.Int("result", a+b))
}

func getFib(ctx context.Context, cl http.Client) (int, error) {
	fib := rand.Intn(30) + 20
	ctx = otel.ContextWithBaggageValues(ctx,
		label.Int("fibInput", fib))
	ctx, span := global.Tracer(serviceName).Start(ctx, "fibClient")
	fmt.Println(span.SpanContext().TraceID.String())
	defer span.End()
	url := fmt.Sprint("http://localhost:9090/fib?n=", fib)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("Error creating http request: %w", err)
	}
	r, err := cl.Do(req)
	if err != nil {
		return 0, fmt.Errorf("Error requesting fib number: %w", err)
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		return 0, errors.New("Not OK response, error code: " + r.Status)
	}
	body, _ := ioutil.ReadAll(r.Body)
	fmt.Printf("Response: %s\n", string(body))
	return strconv.Atoi(string(body))
}

func initHoneycomb() *honeycomb.Exporter {
	ex, err := honeycomb.NewExporter(
		honeycomb.Config{
			APIKey: os.Getenv("HONEYCOMB_KEY"),
		},
		honeycomb.TargetingDataset("opentelemetry"))
	if err != nil {
		panic(err)
	}
	return ex
}

func initTracer(exporter *honeycomb.Exporter) func() {
	bsp := sdktrace.NewBatchSpanProcessor(exporter)
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(bsp), sdktrace.WithSyncer(exporter))
	tp.ApplyConfig(
		sdktrace.Config{
			DefaultSampler: sdktrace.AlwaysSample(),
		})
	global.SetTextMapPropagator(
		otel.NewCompositeTextMapPropagator(propagators.Baggage{}, propagators.TraceContext{}),
	)
	global.SetTracerProvider(tp)
	return bsp.Shutdown
}
