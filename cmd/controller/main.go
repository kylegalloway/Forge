package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/kylegalloway/forge/pkg/controller"
	"github.com/kylegalloway/forge/pkg/telemetry"
)

var (
	masterURL   string
	kubeconfig  string
	namespace   string
	healthAddr  string
	metricsAddr string
)

func main() {
	klog.InitFlags(nil)
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&namespace, "namespace", "", "Namespace to watch. Empty string means all namespaces.")
	flag.StringVar(&healthAddr, "health-addr", ":8081", "The address for health check endpoints.")
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address for metrics endpoints.")
	flag.Parse()

	// Set up signals so we handle the first shutdown signal gracefully
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChan
		klog.Info("Received shutdown signal")
		cancel()
	}()

	// Build the Kubernetes config
	cfg, err := buildConfig(kubeconfig, masterURL)
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %v", err)
	}

	// Create the Kubernetes client
	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %v", err)
	}

	// Create the dynamic client
	dynamicClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building dynamic client: %v", err)
	}

	// If namespace is not specified, watch all namespaces
	watchNamespace := namespace
	if watchNamespace == "" {
		watchNamespace = corev1.NamespaceAll
		klog.Info("Watching all namespaces")
	} else {
		klog.Infof("Watching namespace: %s", watchNamespace)
	}

	// Initialize OpenTelemetry metrics
	metrics, err := telemetry.NewMetrics()
	if err != nil {
		klog.Fatalf("Error creating metrics: %v", err)
	}
	klog.Info("OpenTelemetry metrics initialized")

	// Initialize OpenTelemetry tracer
	tracer := telemetry.NewTracer()
	klog.Info("OpenTelemetry tracer initialized")

	// Create the controller with telemetry
	ctrl := controller.NewController(kubeClient, dynamicClient, watchNamespace, metrics, tracer)

	// Start health check server
	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/healthz", ctrl.HealthzHandler())
	healthMux.HandleFunc("/readyz", ctrl.ReadyzHandler())

	healthServer := &http.Server{
		Addr:    healthAddr,
		Handler: healthMux,
	}

	go func() {
		klog.InfoS("Starting health check server", "addr", healthAddr)
		if err := healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			klog.ErrorS(err, "Health check server failed")
		}
	}()

	// Start metrics server with Prometheus exporter
	// Create Prometheus exporter that bridges OTel metrics to Prometheus format
	promExporter, err := prometheus.New()
	if err != nil {
		klog.Fatalf("Error creating Prometheus exporter: %v", err)
	}

	// Create MeterProvider with Prometheus exporter
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(promExporter),
	)
	defer func() {
		if err := meterProvider.Shutdown(ctx); err != nil {
			klog.ErrorS(err, "Error shutting down meter provider")
		}
	}()

	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())

	metricsServer := &http.Server{
		Addr:    metricsAddr,
		Handler: metricsMux,
	}

	go func() {
		klog.InfoS("Starting metrics server (Prometheus-compatible)", "addr", metricsAddr)
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			klog.ErrorS(err, "Metrics server failed")
		}
	}()

	// Run the controller
	klog.Info("Starting ScriptRunner controller")
	if err := ctrl.Run(ctx); err != nil {
		klog.Fatalf("Error running controller: %v", err)
	}

	// Graceful shutdown of servers
	klog.Info("Shutting down servers")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := healthServer.Shutdown(shutdownCtx); err != nil {
		klog.ErrorS(err, "Health server shutdown error")
	}
	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		klog.ErrorS(err, "Metrics server shutdown error")
	}

	klog.Info("ScriptRunner controller stopped")
}

// buildConfig builds a Kubernetes REST config from kubeconfig or in-cluster config
func buildConfig(kubeconfig, masterURL string) (*rest.Config, error) {
	if kubeconfig != "" {
		klog.Infof("Using kubeconfig: %s", kubeconfig)
		return clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	}

	klog.Info("Using in-cluster config")
	return rest.InClusterConfig()
}
