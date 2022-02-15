/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package server

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	resourceVNAgentSubsystem             = "vn_agent"
	metricNameFailureCounterForNamespace = "counter_for_tenant_namespace"
	metricNameFailureCounterForTenants   = "counter_for_tenant_failure"
	metricNameInFlightRequests           = "in_flight_requests"
	metricNameTotalRequests              = "total_requests"
	metricNameRequestLatency             = "request_latencies"
	errorProxyingRequest                 = "error_proxying_request"
	errorTranslatingPath                 = "error_translating_path"
)

var (
	counterForTenantsAndNamespace = prometheus.NewCounterVec(prometheus.CounterOpts{
		Subsystem: resourceVNAgentSubsystem,
		Name:      metricNameFailureCounterForNamespace,
		Help:      "counter for api operations by tenants",
	}, []string{"host", "action", "tenantName", "tenantNamespace", "responseCode"})

	failureCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Subsystem: resourceVNAgentSubsystem,
		Name:      metricNameFailureCounterForTenants,
		Help:      "counter for failure api operations by tenants",
	}, []string{"host", "action", "tenantName", "tenantNamespace", "reason"})

	inFlightRequests = prometheus.NewGauge(prometheus.GaugeOpts{
		Subsystem: resourceVNAgentSubsystem,
		Name:      metricNameInFlightRequests,
		Help:      "inflight requests",
	})

	totalRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Subsystem: resourceVNAgentSubsystem,
		Name:      metricNameTotalRequests,
		Help:      "total requests",
	}, []string{"code", "method"})

	requestLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: resourceVNAgentSubsystem,
			Name:      metricNameRequestLatency,
			Help:      "request latencies",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{},
	)
)

var registerMetrics sync.Once

func register() {
	registerMetrics.Do(func() {
		prometheus.MustRegister(counterForTenantsAndNamespace,
			failureCounter,
			inFlightRequests,
			totalRequests,
			requestLatency)
	})
}

// NewMetricsServer initializes and configures the MetricsServer.
func NewMetricsServer() *http.ServeMux {
	register()
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	return mux
}

func getRoundTripper(transport http.RoundTripper,
	host, tenantName, action, podNamespace string) http.RoundTripper {
	return promhttp.InstrumentRoundTripperInFlight(inFlightRequests,
		promhttp.InstrumentRoundTripperCounter(totalRequests,
			promhttp.InstrumentRoundTripperDuration(requestLatency, agentRoundTripper{
				nextRoundTripper: transport,
				tenantName:       tenantName,
				host:             host,
				action:           action,
				podNamespace:     podNamespace,
			})))
}

type agentRoundTripper struct {
	nextRoundTripper http.RoundTripper
	tenantName       string
	host             string
	action           string
	podNamespace     string
}

func (art agentRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	response, err := art.nextRoundTripper.RoundTrip(r)
	if err == nil {
		counterForTenantsAndNamespace.
			WithLabelValues(art.host, art.action, art.tenantName, art.podNamespace, fmt.Sprint(response.StatusCode)).
			Inc()
	}
	return response, err
}
