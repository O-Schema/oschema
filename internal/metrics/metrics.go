package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	EventsIngested = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "oschema_events_ingested_total",
		Help: "Total number of events ingested",
	}, []string{"source", "type"})

	EventsDeduplicated = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "oschema_events_deduplicated_total",
		Help: "Total number of duplicate events rejected",
	}, []string{"source"})

	EventsDeadLettered = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "oschema_events_dead_lettered_total",
		Help: "Total number of events sent to dead letter",
	}, []string{"source"})

	EventsStored = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "oschema_events_stored_total",
		Help: "Total number of events successfully stored",
	}, []string{"source"})

	EventsRetried = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "oschema_events_retried_total",
		Help: "Total number of event processing retries",
	}, []string{"source"})

	IngestLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "oschema_ingest_duration_seconds",
		Help:    "Histogram of ingest request latency",
		Buckets: prometheus.DefBuckets,
	}, []string{"source", "status"})

	QueueDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "oschema_queue_depth",
		Help: "Current number of messages in the processing queue",
	})

	SignatureFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "oschema_signature_failures_total",
		Help: "Total number of webhook signature verification failures",
	}, []string{"source"})

	IngestErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "oschema_ingest_errors_total",
		Help: "Total number of ingest errors by type",
	}, []string{"source", "error_type"})
)
