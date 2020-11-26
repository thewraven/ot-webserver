package main

import (
	"context"
	"encoding/json"
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
	"github.com/lightstep/otel-launcher-go/launcher"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/api/global"

	"go.opentelemetry.io/otel/label"
)

const (
	serviceName = "otelClient"
	host        = "http://localhost:9090"
)

type User struct {
	ID  string `json:"id"`
	Key string `json:"key"`
}

func main() {
	rand.Seed(time.Now().Unix())
	closeTracer := initTracer()
	defer closeTracer()
	cl := &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}
	ctx, span := global.Tracer(serviceName).Start(context.Background(), "mathClient")
	u, err := login(ctx, *cl, "user1")
	if err != nil {
		panic(err)
	}
	addFibs(ctx, *cl, *u)
	span.End()
	time.Sleep(time.Second * 3)
}

func login(ctx context.Context, cl http.Client, u string) (*User, error) {
	ctx, span := global.Tracer(serviceName).Start(ctx, "login")
	defer span.End()
	url := fmt.Sprint(host, "/login?user=", u)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		span.RecordError(ctx, err)
		return nil, err
	}
	r, err := cl.Do(req)
	if err != nil {
		err = fmt.Errorf("Error requesting fib number: %w", err)
		span.RecordError(ctx, err)
		return nil, err
	}
	user := new(User)
	err = json.NewDecoder(r.Body).Decode(user)
	if err != nil {
		span.RecordError(ctx, err)
		return nil, err
	}
	return user, nil
}

func addFibs(ctx context.Context, cl http.Client, u User) {
	ctx, span := global.Tracer(serviceName).Start(ctx, "addFibs")
	defer span.End()
	var wg sync.WaitGroup
	recv := make(chan int)
	wg.Add(2)
	go func(ctx context.Context) {
		defer wg.Done()
		var err error
		ctx, cancel := context.WithTimeout(ctx, time.Second*1)
		defer cancel()
		a, err := getFib(ctx, cl, u)
		if err != nil {
			span.RecordError(ctx, err)
		}
		recv <- a
	}(ctx)
	go func(ctx context.Context) {
		defer wg.Done()
		var err error
		ctx, cancel := context.WithTimeout(ctx, time.Second*1)
		defer cancel()
		b, err := getFib(ctx, cl, u)
		if err != nil {
			span.RecordError(ctx, err)
		}
		recv <- b
	}(ctx)

	r := <-recv + <-recv
	fmt.Println("Result:", r)
	span.AddEventWithTimestamp(ctx, time.Now(), "calculation is over,", label.Int("result", r))
	wg.Wait()
}

func getFib(ctx context.Context, cl http.Client, u User) (int, error) {
	fib := rand.Intn(30) + 20
	ctx = otel.ContextWithBaggageValues(ctx,
		label.Int("fibInput", fib))
	ctx, span := global.Tracer(serviceName).Start(ctx, "fibClient")
	fmt.Println(span.SpanContext().TraceID.String())
	defer span.End()
	url := fmt.Sprint(host, "/fib?n=", fib)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Add("Authorization", u.Key)
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

func initTracer() func() {
	otel := launcher.ConfigureOpentelemetry()
	return func() { otel.Shutdown() }
}
