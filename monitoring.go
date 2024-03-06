package main

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	mutatedCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "toleration_webhook_mutated_total",
			Help: "Total number of k8s objects mutated by the toleration webhook",
		},
		[]string{"objType", "name", "namespace"},
	)
)

func init() {
	// Register the mutatedCounter with Prometheus default registry.
	prometheus.MustRegister(mutatedCounter)
}

func RecordMutatedObject(objType, name, namespace string) {
	mutatedCounter.WithLabelValues(objType, name, namespace).Inc()
}
