package fib

import (
	"context"

	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/label"
)

type Fibber interface {
	Fib(ctx context.Context, n int) int
}

type cachedFib struct {
	impl   Fibber
	values map[int]int
}

//NewCached returns a fibonacci generator, using the default generator
func NewCached() Fibber {
	return cachedFib{impl: New(), values: make(map[int]int)}
}

//Fib returns the nth fibonacci number
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

//New returns an untraced fibonacci generator
func New() Fibber {
	return mathFib{}
}

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

//NewWithTracing instruments a fibonacci generator
func NewWithTracing(f Fibber) Fibber {
	return otFib{ot: f}
}

func (o otFib) Fib(ctx context.Context, n int) int {
	span := trace.SpanFromContext(ctx)
	_, span = span.Tracer().Start(ctx, "/fib invocation")
	defer span.End()
	span.SetAttributes(label.Int("Fib requested", n))
	return o.ot.Fib(ctx, n)
}
