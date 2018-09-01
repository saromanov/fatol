package fatol

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	normMean   float64 = 0.00001
	normDomain float64 = 0.0002
)

var (
	rpcDurationsHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "durations_seconds",
		Help:    "fatol requests",
		Buckets: prometheus.LinearBuckets(normMean-5*normDomain, .5*normDomain, 20),
	})
)

func init() {
	prometheus.MustRegister(rpcDurationsHistogram)
}
