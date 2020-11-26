package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/exporters/trace/jaeger"

	"go.opentelemetry.io/otel/label"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

const (
	serviceName = "otelClient"
)

func main() {
	rand.Seed(time.Now().Unix())
	cl := &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}
	getFib(context.Background(), cl)
}

func getFib(ctx context.Context, cl *http.Client) {
	fib := rand.Intn(30) + 20
	ctx = baggage.ContextWithValues(ctx,
		label.Int("fibInput", fib))
	ctx, span := otel.Tracer(serviceName).Start(ctx, "/fibInvoke")
	defer span.End()
	url := fmt.Sprint("http://localhost:9090/fib?n=", fib)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		panic(err)
	}
	_, req = otelhttptrace.W3C(ctx, req)
	r, err := cl.Do(req)
	if err != nil {
		panic(err)
	}
	if r.StatusCode != http.StatusOK {
		fmt.Println("Not OK response, error code:", r.StatusCode)
	}
	defer r.Body.Close()
	body, _ := ioutil.ReadAll(r.Body)
	fmt.Printf("Response: %s\n", string(body))
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
