package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/honeycombio/opentelemetry-exporter-go/honeycomb"
	"github.com/thewraven/ot-webserver/cache"
	"github.com/thewraven/ot-webserver/fib"
	"github.com/thewraven/ot-webserver/sqlite"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/label"
	"go.opentelemetry.io/otel/propagators"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

var serviceName = os.Getenv("SERVICE_NAME")
var addr = ":9090"

func addInstrumentation(name string, fn http.HandlerFunc) http.Handler {
	return otelhttp.NewHandler(http.HandlerFunc(fn), name)
}

func serveFib(o fib.Fibber, s Session) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := global.Tracer(serviceName).Start(r.Context(), "fibService")
		defer span.End()
		k := r.Header.Get("Authorization")
		id, err := s.Get(ctx, k)
		if err != nil || len(id) == 0 {
			span.RecordError(ctx, err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		span.SetAttributes(label.String("user", string(id)))
		n := r.URL.Query().Get("n")
		f, err := strconv.Atoi(n)
		if err != nil {
			span.RecordError(ctx, err)
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, "Input is not a number", n)
			return
		}
		out := o.Fib(r.Context(), f)
		fmt.Fprint(w, out)
	}
}

func login(s Session, conn *sqlite.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := global.Tracer(serviceName).Start(r.Context(), "login")
		defer span.End()
		u := r.URL.Query().Get("user")
		user, err := conn.FindUser(ctx, u)
		if err != nil {
			span.RecordError(ctx, err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, err.Error())
			return
		}
		s.Save(ctx, user.Key, []byte(user.ID))
		err = json.NewEncoder(w).Encode(user)
		if err != nil {
			span.RecordError(ctx, err)
		}
	}
}

func main() {
	cl := initTracer(initHoneycomb())
	defer cl()
	mux := http.NewServeMux()
	fib := fib.NewWithTracing(fib.NewCached(fib.New()))
	sess := initSession()
	db, err := sqlite.New()
	if err != nil {
		panic(err)
	}
	err = db.FillDB(context.Background())
	if err != nil {
		panic(err)
	}
	mux.Handle("/fib", addInstrumentation("fibEndpoint", serveFib(fib, sess)))
	mux.Handle("/login", addInstrumentation("login", login(sess, db)))
	log.Println("Listening at address", addr)
	http.ListenAndServe(addr, mux)
}

func initSession() Session {
	return cache.NewSession("localhost:11211", "sessionService")
}

func initHoneycomb() *honeycomb.Exporter {
	ex, err := honeycomb.NewExporter(
		honeycomb.Config{
			APIKey: os.Getenv("HONEYCOMB_KEY"),
		})
	if err != nil {
		panic(err)
	}
	return ex
}

func initTracer(exporter *honeycomb.Exporter) func() {
	bsp := sdktrace.NewBatchSpanProcessor(exporter)
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(bsp))
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

type Session interface {
	Save(ctx context.Context, k string, v []byte) error
	Get(ctx context.Context, k string) ([]byte, error)
	Drop(ctx context.Context, k string) error
}
