module github.com/thewraven/ot-webserver

go 1.15

require (
	github.com/bradfitz/gomemcache v0.0.0-20190913173617-a41fca850d0b
	github.com/google/go-cmp v0.5.3 // indirect
	github.com/honeycombio/opentelemetry-exporter-go v0.13.0
	github.com/jinzhu/gorm v1.9.16
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/smacker/opentracing-gorm v0.0.0-20181207094635-cd4974441042
	go.opentelemetry.io/contrib v0.13.0
	go.opentelemetry.io/contrib/instrumentation/github.com/bradfitz/gomemcache/memcache/otelmemcache v0.13.0
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.13.0
	go.opentelemetry.io/otel v0.13.0
	go.opentelemetry.io/otel/sdk v0.13.0
	gorm.io/driver/sqlite v1.1.3
)
