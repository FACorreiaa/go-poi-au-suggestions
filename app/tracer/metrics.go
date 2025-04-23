package tracer

import (
	"log" // For fatal error on metric init failure

	"go.opentelemetry.io/otel/metric"
)

var (
	registerRequestsTotal   metric.Int64Counter
	registerDurationSeconds metric.Float64Histogram
	dbQueryDurationSeconds  metric.Float64Histogram
	dbQueryErrorsTotal      metric.Int64Counter
	// Add other metrics...
)

// InitializeMetrics sets up the application's metrics. Call this during startup.
func InitializeMetrics(meter metric.Meter) { // Pass the meter obtained from MeterProvider
	var err error

	registerRequestsTotal, err = meter.Int64Counter(
		"register_requests_total",
		metric.WithDescription("Total number of register requests"),
	)
	if err != nil {
		log.Fatalf("Failed to create register_requests_total counter: %v", err)
	}

	registerDurationSeconds, err = meter.Float64Histogram(
		"register_duration_seconds",
		metric.WithDescription("Duration of register requests in seconds"),
		metric.WithUnit("s"), // Specify units
	)
	if err != nil {
		log.Fatalf("Failed to create register_duration_seconds histogram: %v", err)
	}

	dbQueryDurationSeconds, err = meter.Float64Histogram(
		"db_query_duration_seconds",
		metric.WithDescription("Duration of database queries in seconds"),
		metric.WithUnit("s"), // Specify units
	)
	if err != nil {
		log.Fatalf("Failed to create db_query_duration_seconds histogram: %v", err)
	}

	dbQueryErrorsTotal, err = meter.Int64Counter(
		"db_query_errors_total",
		metric.WithDescription("Total number of database query errors"),
	)
	if err != nil {
		log.Fatalf("Failed to create db_query_errors_total counter: %v", err)
	}

	log.Println("Application metrics initialized successfully.")
}
