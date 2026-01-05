// Package main implements the Forge controller that watches ZarfPackageJob resources and orchestrates package operations.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/kylegalloway/forge/pkg/controller"
	"github.com/kylegalloway/forge/pkg/leaderelection"
	"github.com/kylegalloway/forge/pkg/logging"
	"github.com/kylegalloway/forge/pkg/telemetry"
)

var (
	masterURL            string
	kubeconfig           string
	namespace            string
	healthAddr           string
	metricsAddr          string
	enableLeaderElection bool
)

func main() {
	parseFlags()

	// Initialize structured logger
	logger := logging.NewLogger("forge-controller")
	ctx := setupSignalHandler(logger)

	cfg := mustBuildConfig(logger)
	kubeClient, dynamicClient := mustCreateClients(cfg, logger)
	watchNamespace := determineWatchNamespace(logger)

	metrics, tracer := initializeTelemetry(logger)

	// Create both Zarf and UDS controllers using generic controller pattern
	zarfCtrl := controller.NewGenericZarfController(kubeClient, dynamicClient, watchNamespace, metrics, tracer)
	udsCtrl := controller.NewGenericUDSController(kubeClient, dynamicClient, watchNamespace, metrics, tracer)

	healthServer := startHealthServer(zarfCtrl, udsCtrl, logger)
	meterProvider, metricsServer := startMetricsServer(ctx, logger)
	defer shutdownMeterProvider(ctx, meterProvider, logger)

	runControllers(ctx, zarfCtrl, udsCtrl, kubeClient, logger)
	shutdownServers(healthServer, metricsServer, logger)
}

func parseFlags() {
	klog.InitFlags(nil)
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&namespace, "namespace", "", "Namespace to watch. Empty string means all namespaces.")
	flag.StringVar(&healthAddr, "health-addr", ":8081", "The address for health check endpoints.")
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address for metrics endpoints.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false, "Enable leader election for high availability.")
	flag.Parse()
}

func setupSignalHandler(logger *logging.Logger) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChan
		logger.Info(ctx, "Received shutdown signal")
		cancel()
	}()
	return ctx
}

func mustBuildConfig(logger *logging.Logger) *rest.Config {
	cfg, err := buildConfig(kubeconfig, masterURL, logger)
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %v", err)
	}
	return cfg
}

func mustCreateClients(cfg *rest.Config, logger *logging.Logger) (*kubernetes.Clientset, dynamic.Interface) {
	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %v", err)
	}
	dynamicClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building dynamic client: %v", err)
	}
	_ = logger // Will use in future error handling
	return kubeClient, dynamicClient
}

func determineWatchNamespace(logger *logging.Logger) string {
	ctx := context.Background()
	watchNamespace := namespace
	if watchNamespace == "" {
		watchNamespace = corev1.NamespaceAll
		logger.Info(ctx, "Watching all namespaces")
	} else {
		logger.Info(ctx, "Watching namespace", "namespace", watchNamespace)
	}
	return watchNamespace
}

func initializeTelemetry(logger *logging.Logger) (*telemetry.Metrics, *telemetry.Tracer) {
	ctx := context.Background()
	metrics, err := telemetry.NewMetrics()
	if err != nil {
		klog.Fatalf("Error creating metrics: %v", err)
	}
	logger.Info(ctx, "OpenTelemetry metrics initialized")

	tracer := telemetry.NewTracer()
	logger.Info(ctx, "OpenTelemetry tracer initialized")
	return metrics, tracer
}

func startHealthServer(zarfCtrl, udsCtrl interface {
	Healthy() bool
	Ready() bool
}, logger *logging.Logger) *http.Server {
	ctx := context.Background()
	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		// Both controllers must be healthy
		if zarfCtrl.Healthy() && udsCtrl.Healthy() {
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte("OK")); err != nil {
				klog.V(5).ErrorS(err, "Failed to write health check response")
			}
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			if _, err := w.Write([]byte("Not healthy")); err != nil {
				klog.V(5).ErrorS(err, "Failed to write health check response")
			}
		}
	})
	healthMux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		// Both controllers must be ready
		if zarfCtrl.Ready() && udsCtrl.Ready() {
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte("Ready")); err != nil {
				klog.V(5).ErrorS(err, "Failed to write ready check response")
			}
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			if _, err := w.Write([]byte("Not ready")); err != nil {
				klog.V(5).ErrorS(err, "Failed to write ready check response")
			}
		}
	})

	healthServer := &http.Server{
		Addr:              healthAddr,
		Handler:           healthMux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info(ctx, "Starting health check server", "addr", healthAddr)
		if serverErr := healthServer.ListenAndServe(); serverErr != nil && serverErr != http.ErrServerClosed {
			logger.Error(ctx, serverErr, "Health check server failed")
		}
	}()

	return healthServer
}

func startMetricsServer(ctx context.Context, logger *logging.Logger) (*sdkmetric.MeterProvider, *http.Server) {
	promExporter, err := prometheus.New()
	if err != nil {
		klog.Fatalf("Error creating Prometheus exporter: %v", err)
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(promExporter),
	)

	// Set as global meter provider so metrics are actually exported
	otel.SetMeterProvider(meterProvider)

	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())

	metricsServer := &http.Server{
		Addr:              metricsAddr,
		Handler:           metricsMux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info(ctx, "Starting metrics server (Prometheus-compatible)", "addr", metricsAddr)
		if serverErr := metricsServer.ListenAndServe(); serverErr != nil && serverErr != http.ErrServerClosed {
			logger.Error(ctx, serverErr, "Metrics server failed")
		}
	}()

	return meterProvider, metricsServer
}

func shutdownMeterProvider(ctx context.Context, meterProvider *sdkmetric.MeterProvider, logger *logging.Logger) {
	if shutdownErr := meterProvider.Shutdown(ctx); shutdownErr != nil {
		logger.Error(ctx, shutdownErr, "Error shutting down meter provider")
	}
}

func runControllers(ctx context.Context, zarfCtrl, udsCtrl interface{ Run(context.Context) error }, kubeClient *kubernetes.Clientset, logger *logging.Logger) {
	logger.Info(ctx, "Starting Forge controllers (Zarf + UDS)")

	if enableLeaderElection {
		logger.Info(ctx, "Leader election enabled")
		leConfig := leaderelection.DefaultConfig()
		err := leaderelection.RunWithLeaderElection(ctx, kubeClient, leConfig, func(ctx context.Context) {
			// Run both controllers concurrently
			errChan := make(chan error, 2)

			go func() {
				logger.Info(ctx, "Starting Zarf controller with leader election")
				if runErr := zarfCtrl.Run(ctx); runErr != nil {
					errChan <- fmt.Errorf("zarf controller error: %w", runErr)
				}
			}()

			go func() {
				logger.Info(ctx, "Starting UDS controller with leader election")
				if runErr := udsCtrl.Run(ctx); runErr != nil {
					errChan <- fmt.Errorf("uds controller error: %w", runErr)
				}
			}()

			// Wait for first controller to fail
			err := <-errChan
			logger.Error(ctx, err, "Controller failed")
		})
		if err != nil {
			klog.Fatalf("Error in leader election: %v", err)
		}
	} else {
		logger.Info(ctx, "Leader election disabled - running both controllers as single instance")

		// Run both controllers concurrently
		errChan := make(chan error, 2)

		go func() {
			logger.Info(ctx, "Starting Zarf controller")
			if runErr := zarfCtrl.Run(ctx); runErr != nil {
				errChan <- fmt.Errorf("zarf controller error: %w", runErr)
			}
		}()

		go func() {
			logger.Info(ctx, "Starting UDS controller")
			if runErr := udsCtrl.Run(ctx); runErr != nil {
				errChan <- fmt.Errorf("uds controller error: %w", runErr)
			}
		}()

		// Wait for first controller to fail
		err := <-errChan
		klog.Fatalf("Controller failed: %v", err)
	}
}

func shutdownServers(healthServer, metricsServer *http.Server, logger *logging.Logger) {
	ctx := context.Background()
	logger.Info(ctx, "Shutting down servers")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := healthServer.Shutdown(shutdownCtx); err != nil {
		logger.Error(ctx, err, "Health server shutdown error")
	}
	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		logger.Error(ctx, err, "Metrics server shutdown error")
	}

	logger.Info(ctx, "Forge controller stopped")
}

// buildConfig builds a Kubernetes REST config from kubeconfig or in-cluster config
func buildConfig(kubeconfig, masterURL string, logger *logging.Logger) (*rest.Config, error) {
	ctx := context.Background()
	if kubeconfig != "" {
		logger.Info(ctx, "Using kubeconfig", "path", kubeconfig)
		return clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	}

	logger.Info(ctx, "Using in-cluster config")
	return rest.InClusterConfig()
}
