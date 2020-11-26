module github.com/thewraven/ot-webserver

go 1.15

require (
	github.com/bradfitz/gomemcache v0.0.0-20190913173617-a41fca850d0b
	github.com/google/uuid v1.1.2
	github.com/honeycombio/opentelemetry-exporter-go v0.13.0
	github.com/jinzhu/gorm v1.9.16
	github.com/lightstep/otel-launcher-go v0.13.0
	github.com/sethvargo/go-envconfig v0.3.2 // indirect
	github.com/thewraven/otgorm v0.13.0
	go.opentelemetry.io/collector v0.15.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/github.com/bradfitz/gomemcache/memcache/otelmemcache v0.13.0
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.13.0
	go.opentelemetry.io/otel v0.13.0
	golang.org/x/net v0.0.0-20201110031124-69a78807bb2b // indirect
	golang.org/x/sys v0.0.0-20201126144705-a4b67b81d3d2 // indirect
	google.golang.org/genproto v0.0.0-20201119123407-9b1e624d6bc4 // indirect
	gorm.io/driver/sqlite v1.1.3
)
