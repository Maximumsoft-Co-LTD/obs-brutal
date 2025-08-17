package shared

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/trace"
)

// SafePrometheus contains the metrics with graceful degradation
type SafePrometheus struct {
	enabled       bool
	reqDur        *prometheus.HistogramVec
	reqCount      *prometheus.CounterVec
	router        *gin.Engine
	listenAddress string
	MetricsPath   string
	service       string
	environment   string
}

// NewSafePrometheusWith creates Prometheus with service/environment labels and graceful degradation
func NewSafePrometheusWith(subsystem, service, environment string, enabled bool) *SafePrometheus {
	p := &SafePrometheus{
		enabled:     enabled,
		MetricsPath: "/metrics",
		service:     service,
		environment: environment,
	}

	if enabled {
		if err := p.registerMetrics(subsystem); err != nil {
			fmt.Printf("Warning: Failed to register Prometheus metrics: %v. Metrics disabled.\n", err)
			p.enabled = false
		} else {
			fmt.Println("Info: Prometheus metrics initialized successfully.")
		}
	} else {
		fmt.Println("Info: Prometheus metrics disabled in configuration.")
	}

	return p
}

func (p *SafePrometheus) registerMetrics(subsystem string) error {
	if !p.enabled {
		return nil
	}

	p.reqDur = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: subsystem,
			Name:      "request_duration_seconds",
			Help:      "Histogram request latencies",
			Buckets:   []float64{.005, .01, .02, 0.04, .06, 0.08, .1, 0.15, .25, 0.4, .6, .8, 1, 1.5, 2, 3, 5},
		},
		[]string{"code", "method", "path", "service", "environment"},
	)

	p.reqCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Subsystem: subsystem,
		Name:      "http_requests_total",
		Help:      "Total number of HTTP requests.",
	}, []string{"code", "method", "path", "service", "environment"})

	if err := prometheus.Register(p.reqDur); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			return fmt.Errorf("failed to register duration histogram: %w", err)
		}
	}

	if err := prometheus.Register(p.reqCount); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			return fmt.Errorf("failed to register request counter: %w", err)
		}
	}

	return nil
}

// SetListenAddress sets the address for exposing metrics
func (p *SafePrometheus) SetListenAddress(address string) {
	if !p.enabled {
		return
	}
	p.listenAddress = address
	if p.listenAddress != "" {
		p.router = gin.Default()
	}
}

// SetMetricsPath sets up the metrics endpoint
func (p *SafePrometheus) SetMetricsPath(e *gin.Engine) {
	if !p.enabled {
		return
	}
	if p.listenAddress != "" {
		p.router.GET(p.MetricsPath, p.prometheusHandler())
		go func() {
			if err := p.router.Run(p.listenAddress); err != nil {
				fmt.Printf("Warning: Metrics server failed to start on %s: %v\n", p.listenAddress, err)
			}
		}()
	} else {
		e.GET(p.MetricsPath, p.prometheusHandler())
	}
}

// Use adds the middleware to a gin engine
func (p *SafePrometheus) Use(e *gin.Engine) {
	e.Use(p.HandlerFunc())
	if p.enabled {
		e.GET(p.MetricsPath, p.prometheusHandler())
	}
}

// HandlerFunc returns the middleware handler function
func (p *SafePrometheus) HandlerFunc() gin.HandlerFunc {
	if !p.enabled {
		return func(c *gin.Context) { c.Next() }
	}

	return func(c *gin.Context) {
		if c.Request.URL.String() == p.MetricsPath {
			c.Next()
			return
		}

		path := c.FullPath()
		if path == "" {
			c.Next()
			return
		}

		method := c.Request.Method
		start := time.Now()
		c.Next()

		status := strconv.Itoa(c.Writer.Status())
		tid := ""
		if span := trace.SpanFromContext(c.Request.Context()); span.SpanContext().IsValid() {
			tid = span.SpanContext().TraceID().String()
		}

		svc := p.service
		if svc == "" {
			svc = "unknown"
		}
		env := p.environment
		if env == "" {
			env = "unknown"
		}

		if p.reqCount != nil {
			ctr := p.reqCount.WithLabelValues(status, method, path, svc, env)
			if adder, ok := ctr.(prometheus.ExemplarAdder); ok && tid != "" {
				adder.AddWithExemplar(1, prometheus.Labels{"trace_id": tid})
			} else {
				ctr.Inc()
			}
		}

		if p.reqDur != nil {
			durSec := time.Since(start).Seconds()
			hist := p.reqDur.WithLabelValues(status, method, path, svc, env)
			if obs, ok := hist.(prometheus.ExemplarObserver); ok && tid != "" {
				obs.ObserveWithExemplar(durSec, prometheus.Labels{"trace_id": tid})
			} else {
				hist.Observe(durSec)
			}
		}
	}
}

func (p *SafePrometheus) prometheusHandler() gin.HandlerFunc {
	if !p.enabled {
		return func(c *gin.Context) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Metrics disabled"})
		}
	}
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

// r := obsv.NewGinEngine()

// m := obsv.NewSafePrometheusWith("http", "my-svc", "prod", true)
// // ทั้ง middleware + GET /metrics (path เริ่มต้นคือ /metrics)
// m.Use(r)

// r.Run(":8080")

// r := obsv.NewGinEngine()

// m := obsv.NewSafePrometheusWith("http", "my-svc", "prod", true)
// // ทั้ง middleware + GET /metrics (path เริ่มต้นคือ /metrics)
// m.Use(r)

// r.Run(":8080")

// m := obsv.NewSafePrometheusWith("http", "my-svc", "prod", true)
// r := obsv.NewGinEngine()

// r.Use(m.HandlerFunc())

// // เปิดเซิร์ฟเวอร์ metrics แยกที่ :9090
// m.SetListenAddress(":9090")
// m.SetMetricsPath(r) // จะ start server แยกและ map /metrics ให้อัตโนมัติ
