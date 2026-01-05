// Package main implements the admission webhook server for validating ZarfPackageJob resources.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	udsv1alpha2 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha2"
	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
	"github.com/kylegalloway/forge/pkg/logging"
	"github.com/kylegalloway/forge/pkg/webhook"
)

var (
	certFile string
	keyFile  string
	port     int
	scheme   = runtime.NewScheme()
	codecs   = serializer.NewCodecFactory(scheme)
)

func init() {
	if err := admissionv1.AddToScheme(scheme); err != nil {
		panic(fmt.Sprintf("failed to add admissionv1 to scheme: %v", err))
	}
	if err := zarfv1alpha1.AddToScheme(scheme); err != nil {
		panic(fmt.Sprintf("failed to add zarfv1alpha1 to scheme: %v", err))
	}
	if err := udsv1alpha2.AddToScheme(scheme); err != nil {
		panic(fmt.Sprintf("failed to add udsv1alpha2 to scheme: %v", err))
	}
}

func main() {
	klog.InitFlags(nil)
	flag.StringVar(&certFile, "tls-cert-file", "/etc/webhook/certs/tls.crt", "TLS certificate file")
	flag.StringVar(&keyFile, "tls-key-file", "/etc/webhook/certs/tls.key", "TLS key file")
	flag.IntVar(&port, "port", 8443, "Webhook server port")
	flag.Parse()

	// Initialize structured logger
	logger := logging.NewLogger("forge-webhook")
	ctx := context.Background()

	// Create in-cluster Kubernetes client
	config, err := rest.InClusterConfig()
	if err != nil {
		klog.Fatalf("Failed to create in-cluster config: %v", err)
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	// Create validators
	zarfValidator := webhook.NewZarfPackageJobValidator(kubeClient)
	udsValidator := webhook.NewUDSBundleJobValidator(kubeClient)

	// Create webhook server
	server := &WebhookServer{
		zarfValidator: zarfValidator,
		udsValidator:  udsValidator,
		logger:        logger,
	}

	// Set up HTTP handlers
	mux := http.NewServeMux()
	mux.HandleFunc("/validate", server.serveValidate)
	mux.HandleFunc("/healthz", server.serveHealthz)
	mux.HandleFunc("/readyz", server.serveReadyz)

	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Set up graceful shutdown
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-signalChan
		logger.Info(ctx, "Received shutdown signal")

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			logger.Error(ctx, err, "Error during shutdown")
		}
	}()

	// Start server
	logger.Info(ctx, "Starting Forge webhook server", "port", port)
	if err := httpServer.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
		klog.Fatalf("Failed to start webhook server: %v", err)
	}

	logger.Info(ctx, "Webhook server stopped")
}

// WebhookServer handles admission webhook requests for ZarfPackageJob and UDSBundleJob resources
type WebhookServer struct {
	zarfValidator *webhook.ZarfPackageJobValidator
	udsValidator  *webhook.UDSBundleJobValidator
	logger        *logging.Logger
}

// serveValidate handles validation webhook requests
func (ws *WebhookServer) serveValidate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ws.logger.Trace(ctx, "Received validation request", "method", r.Method, "url", r.URL.String())

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		ws.logger.Error(ctx, err, "Failed to read request body")
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	// Verify content type
	if r.Header.Get("Content-Type") != "application/json" {
		ws.logger.Warning(ctx, "Invalid content type", "contentType", r.Header.Get("Content-Type"))
		http.Error(w, "invalid content type, expected application/json", http.StatusBadRequest)
		return
	}

	// Decode admission review
	admissionReview := admissionv1.AdmissionReview{}
	deserializer := codecs.UniversalDeserializer()
	if _, _, decodeErr := deserializer.Decode(body, nil, &admissionReview); decodeErr != nil {
		ws.logger.Error(ctx, decodeErr, "Failed to decode admission review")
		http.Error(w, fmt.Sprintf("failed to decode admission review: %v", decodeErr), http.StatusBadRequest)
		return
	}

	// Process admission request
	response := ws.validate(r.Context(), admissionReview.Request)

	// Create admission review response
	admissionReview.Response = response
	admissionReview.Response.UID = admissionReview.Request.UID

	// Encode and send response
	responseBytes, err := json.Marshal(admissionReview)
	if err != nil {
		ws.logger.Error(ctx, err, "Failed to encode response")
		http.Error(w, fmt.Sprintf("failed to encode response: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(responseBytes); err != nil {
		ws.logger.Error(ctx, err, "Failed to write response")
	}
}

// validate performs the actual validation
func (ws *WebhookServer) validate(ctx context.Context, request *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	// Determine resource type and route to appropriate validator
	switch request.Kind.Kind {
	case "ZarfPackageJob":
		return ws.validateZarfPackageJob(ctx, request)
	case "UDSBundleJob":
		return ws.validateUDSBundleJob(ctx, request)
	default:
		return &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: fmt.Sprintf("unknown resource kind: %s", request.Kind.Kind),
			},
		}
	}
}

// validateZarfPackageJob validates a ZarfPackageJob resource
func (ws *WebhookServer) validateZarfPackageJob(ctx context.Context, request *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	ws.logger.Info(ctx, "Validating ZarfPackageJob",
		"name", request.Name,
		"namespace", request.Namespace,
		"operation", request.Operation,
		"user", request.UserInfo.Username)

	// Decode ZarfPackageJob object
	var pkg zarfv1alpha1.ZarfPackageJob
	deserializer := codecs.UniversalDeserializer()
	if _, _, err := deserializer.Decode(request.Object.Raw, nil, &pkg); err != nil {
		ws.logger.Error(ctx, err, "Failed to decode ZarfPackageJob")
		return &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: fmt.Sprintf("failed to decode ZarfPackageJob: %v", err),
			},
		}
	}

	// Validate the ZarfPackageJob against ServiceAccount permissions
	if err := ws.zarfValidator.ValidateZarfPackageJob(ctx, &pkg); err != nil {
		ws.logger.Info(ctx, "Validation failed",
			"name", pkg.Name,
			"namespace", pkg.Namespace,
			"error", err.Error())

		return &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: fmt.Sprintf("ZarfPackageJob validation failed: %v", err),
			},
		}
	}

	ws.logger.Info(ctx, "Validation succeeded",
		"name", pkg.Name,
		"namespace", pkg.Namespace)

	return &admissionv1.AdmissionResponse{
		Allowed: true,
	}
}

// validateUDSBundleJob validates a UDSBundleJob resource
func (ws *WebhookServer) validateUDSBundleJob(ctx context.Context, request *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	ws.logger.Info(ctx, "Validating UDSBundleJob",
		"name", request.Name,
		"namespace", request.Namespace,
		"operation", request.Operation,
		"user", request.UserInfo.Username)

	// Decode UDSBundleJob object
	var bundle udsv1alpha2.UDSBundleJob
	deserializer := codecs.UniversalDeserializer()
	if _, _, err := deserializer.Decode(request.Object.Raw, nil, &bundle); err != nil {
		ws.logger.Error(ctx, err, "Failed to decode UDSBundleJob")
		return &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: fmt.Sprintf("failed to decode UDSBundleJob: %v", err),
			},
		}
	}

	// Validate the UDSBundleJob against ServiceAccount permissions
	if err := ws.udsValidator.ValidateUDSBundleJob(ctx, &bundle); err != nil {
		ws.logger.Info(ctx, "Validation failed",
			"name", bundle.Name,
			"namespace", bundle.Namespace,
			"error", err.Error())

		return &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: fmt.Sprintf("UDSBundleJob validation failed: %v", err),
			},
		}
	}

	ws.logger.Info(ctx, "Validation succeeded",
		"name", bundle.Name,
		"namespace", bundle.Namespace)

	return &admissionv1.AdmissionResponse{
		Allowed: true,
	}
}

// serveHealthz handles health check requests
func (ws *WebhookServer) serveHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("ok")); err != nil {
		klog.ErrorS(err, "Failed to write health response")
	}
}

// serveReadyz handles readiness check requests
func (ws *WebhookServer) serveReadyz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("ready")); err != nil {
		klog.ErrorS(err, "Failed to write ready response")
	}
}
