package cache

import (
	"context"

	"github.com/bradfitz/gomemcache/memcache"
	"go.opentelemetry.io/contrib/instrumentation/github.com/bradfitz/gomemcache/memcache/otelmemcache"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/label"
)

type cache struct {
	mem *otelmemcache.Client
}

func NewSession(addr string, service string) *cache {
	cl := memcache.New(addr)
	ct := otelmemcache.NewClientWithTracing(cl, otelmemcache.WithServiceName(service))
	return &cache{mem: ct}
}

func (c *cache) Save(ctx context.Context, k string, v []byte) error {
	item := memcache.Item{Key: k, Value: v}
	err := c.mem.WithContext(ctx).Add(&item)
	if err != nil {
		span := trace.SpanFromContext(ctx)
		defer span.End(trace.WithRecord())
		span.RecordError(ctx, err)
		return err
	}
	return nil
}

func (c *cache) Get(ctx context.Context, k string) ([]byte, error) {
	i, err := c.mem.WithContext(ctx).Get(k)
	if err != nil {
		span := trace.SpanFromContext(ctx)
		defer span.End(trace.WithRecord())
		span.RecordError(ctx, err)
		return nil, err
	}
	return i.Value, nil
}

func (c *cache) Drop(ctx context.Context, k string) error {
	err := c.mem.WithContext(ctx).Delete(k)
	if err != nil {
		span := trace.SpanFromContext(ctx)
		defer span.End(trace.WithRecord())
		span.SetAttributes(label.String("undeletedKey", k))
		span.RecordError(ctx, err)
		return err
	}
	return nil
}
