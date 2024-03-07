package main

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	mutatedCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "toleration_webhook_total",
			Help: "Total number of k8s objects mutated by the toleration webhook",
		},
		[]string{"event_type", "obj_type", "name", "namespace", "mutated"},
	)
)

func init() {
	// Register the mutatedCounter with Prometheus default registry.
	prometheus.MustRegister(mutatedCounter)
}

func RecordObject(event_type, obj_type, name, namespace, mutated string) {
	mutatedCounter.WithLabelValues(event_type, obj_type, name, namespace, mutated).Inc()
}
