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
	"github.com/lightstep/otel-launcher-go/launcher"
	"github.com/thewraven/ot-webserver/cache"
	"github.com/thewraven/ot-webserver/sqlite"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/label"
)

var serviceName = os.Getenv("SERVICE_NAME")
var addr = ":9090"

type Fibber interface {
	Fib(ctx context.Context, n int) int
}

type cachedFib struct {
	impl   Fibber
	values map[int]int
}

func NewCached(real Fibber) Fibber {
	return cachedFib{impl: real, values: make(map[int]int)}
}

func (c cachedFib) Fib(ctx context.Context, n int) int {
	ctx, span := trace.SpanFromContext(ctx).Tracer().Start(ctx, "cachedSpan")
	defer span.End()
	if v, ok := c.values[n]; ok {
		span.AddEvent(ctx, "value cached", label.Int("f", n))
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
	ot Fibber
}

func (o otFib) Fib(ctx context.Context, n int) int {
	span := trace.SpanFromContext(ctx)
	_, span = span.Tracer().Start(ctx, "/fib invocation")
	defer span.End()
	span.SetAttributes(label.Int("Fib requested", n))
	return o.ot.Fib(ctx, n)
}

func (o otFib) serveFib(s Session) http.HandlerFunc {
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

func addInstrumentation(name string, fn http.HandlerFunc) http.Handler {
	return otelhttp.NewHandler(http.HandlerFunc(fn), name)
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
	cl := initTracer()
	defer cl()
	mux := http.NewServeMux()
	cache := NewCached(mathFib{})
	fb := otFib{ot: cache}
	sess := initSession()
	db, err := sqlite.New()
	if err != nil {
		panic(err)
	}
	err = db.FillDB(context.Background())
	if err != nil {
		panic(err)
	}
	mux.Handle("/fib", addInstrumentation("fibEndpoint", fb.serveFib(sess)))
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

func initTracer() func() {
	otel := launcher.ConfigureOpentelemetry()
	return func() { otel.Shutdown() }
}

type Session interface {
	Save(ctx context.Context, k string, v []byte) error
	Get(ctx context.Context, k string) ([]byte, error)
	Drop(ctx context.Context, k string) error
}
