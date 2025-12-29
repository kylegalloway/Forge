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
	ctx := setupSignalHandler()
	cfg := mustBuildConfig()
	kubeClient, dynamicClient := mustCreateClients(cfg)
	watchNamespace := determineWatchNamespace()

	metrics, tracer := initializeTelemetry()

	// Create both Zarf and UDS controllers using generic controller pattern
	zarfCtrl := controller.NewGenericZarfController(kubeClient, dynamicClient, watchNamespace, metrics, tracer)
	udsCtrl := controller.NewGenericUDSController(kubeClient, dynamicClient, watchNamespace, metrics, tracer)

	healthServer := startHealthServer(zarfCtrl, udsCtrl)
	meterProvider, metricsServer := startMetricsServer(ctx)
	defer shutdownMeterProvider(ctx, meterProvider)

	runControllers(ctx, zarfCtrl, udsCtrl, kubeClient)
	shutdownServers(healthServer, metricsServer)
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

func setupSignalHandler() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChan
		klog.Info("Received shutdown signal")
		cancel()
	}()
	return ctx
}

func mustBuildConfig() *rest.Config {
	cfg, err := buildConfig(kubeconfig, masterURL)
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %v", err)
	}
	return cfg
}

func mustCreateClients(cfg *rest.Config) (*kubernetes.Clientset, dynamic.Interface) {
	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %v", err)
	}
	dynamicClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building dynamic client: %v", err)
	}
	return kubeClient, dynamicClient
}

func determineWatchNamespace() string {
	watchNamespace := namespace
	if watchNamespace == "" {
		watchNamespace = corev1.NamespaceAll
		klog.Info("Watching all namespaces")
	} else {
		klog.Infof("Watching namespace: %s", watchNamespace)
	}
	return watchNamespace
}

func initializeTelemetry() (*telemetry.Metrics, *telemetry.Tracer) {
	metrics, err := telemetry.NewMetrics()
	if err != nil {
		klog.Fatalf("Error creating metrics: %v", err)
	}
	klog.Info("OpenTelemetry metrics initialized")

	tracer := telemetry.NewTracer()
	klog.Info("OpenTelemetry tracer initialized")
	return metrics, tracer
}

func startHealthServer(zarfCtrl, udsCtrl interface {
	Healthy() bool
	Ready() bool
}) *http.Server {
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
		klog.InfoS("Starting health check server", "addr", healthAddr)
		if serverErr := healthServer.ListenAndServe(); serverErr != nil && serverErr != http.ErrServerClosed {
			klog.ErrorS(serverErr, "Health check server failed")
		}
	}()

	return healthServer
}

func startMetricsServer(_ context.Context) (*sdkmetric.MeterProvider, *http.Server) {
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
		klog.InfoS("Starting metrics server (Prometheus-compatible)", "addr", metricsAddr)
		if serverErr := metricsServer.ListenAndServe(); serverErr != nil && serverErr != http.ErrServerClosed {
			klog.ErrorS(serverErr, "Metrics server failed")
		}
	}()

	return meterProvider, metricsServer
}

func shutdownMeterProvider(ctx context.Context, meterProvider *sdkmetric.MeterProvider) {
	if shutdownErr := meterProvider.Shutdown(ctx); shutdownErr != nil {
		klog.ErrorS(shutdownErr, "Error shutting down meter provider")
	}
}

func runControllers(ctx context.Context, zarfCtrl, udsCtrl interface{ Run(context.Context) error }, kubeClient *kubernetes.Clientset) {
	klog.Info("Starting Forge controllers (Zarf + UDS)")

	if enableLeaderElection {
		klog.Info("Leader election enabled")
		leConfig := leaderelection.DefaultConfig()
		err := leaderelection.RunWithLeaderElection(ctx, kubeClient, leConfig, func(ctx context.Context) {
			// Run both controllers concurrently
			errChan := make(chan error, 2)

			go func() {
				klog.Info("Starting Zarf controller with leader election")
				if runErr := zarfCtrl.Run(ctx); runErr != nil {
					errChan <- fmt.Errorf("zarf controller error: %w", runErr)
				}
			}()

			go func() {
				klog.Info("Starting UDS controller with leader election")
				if runErr := udsCtrl.Run(ctx); runErr != nil {
					errChan <- fmt.Errorf("uds controller error: %w", runErr)
				}
			}()

			// Wait for first controller to fail
			err := <-errChan
			klog.ErrorS(err, "Controller failed")
		})
		if err != nil {
			klog.Fatalf("Error in leader election: %v", err)
		}
	} else {
		klog.Info("Leader election disabled - running both controllers as single instance")

		// Run both controllers concurrently
		errChan := make(chan error, 2)

		go func() {
			klog.Info("Starting Zarf controller")
			if runErr := zarfCtrl.Run(ctx); runErr != nil {
				errChan <- fmt.Errorf("zarf controller error: %w", runErr)
			}
		}()

		go func() {
			klog.Info("Starting UDS controller")
			if runErr := udsCtrl.Run(ctx); runErr != nil {
				errChan <- fmt.Errorf("uds controller error: %w", runErr)
			}
		}()

		// Wait for first controller to fail
		err := <-errChan
		klog.Fatalf("Controller failed: %v", err)
	}
}

func shutdownServers(healthServer, metricsServer *http.Server) {
	klog.Info("Shutting down servers")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := healthServer.Shutdown(shutdownCtx); err != nil {
		klog.ErrorS(err, "Health server shutdown error")
	}
	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		klog.ErrorS(err, "Metrics server shutdown error")
	}

	klog.Info("Forge controller stopped")
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
