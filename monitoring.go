package main

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	mutatedCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "toleration_webhook_mutated_total",
			Help: "Total number of objects mutated by the webhook",
		},
		[]string{"k8sObjectName"},
	)
)

func init() {
	// Register the mutatedCounter with Prometheus default registry.
	prometheus.MustRegister(mutatedCounter)
}

func RecordMutatedObject(k8sObjectName string) {
	mutatedCounter.WithLabelValues(k8sObjectName).Inc()
}
