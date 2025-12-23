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

	udsv1alpha1 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha1"
	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
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
	if err := udsv1alpha1.AddToScheme(scheme); err != nil {
		panic(fmt.Sprintf("failed to add udsv1alpha1 to scheme: %v", err))
	}
}

func main() {
	klog.InitFlags(nil)
	flag.StringVar(&certFile, "tls-cert-file", "/etc/webhook/certs/tls.crt", "TLS certificate file")
	flag.StringVar(&keyFile, "tls-key-file", "/etc/webhook/certs/tls.key", "TLS key file")
	flag.IntVar(&port, "port", 8443, "Webhook server port")
	flag.Parse()

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
		klog.Info("Received shutdown signal")

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			klog.ErrorS(err, "Error during shutdown")
		}
	}()

	// Start server
	klog.InfoS("Starting Forge webhook server", "port", port)
	if err := httpServer.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
		klog.Fatalf("Failed to start webhook server: %v", err)
	}

	klog.Info("Webhook server stopped")
}

// WebhookServer handles admission webhook requests for ZarfPackageJob and UDSBundleJob resources
type WebhookServer struct {
	zarfValidator *webhook.ZarfPackageJobValidator
	udsValidator  *webhook.UDSBundleJobValidator
}

// serveValidate handles validation webhook requests
func (ws *WebhookServer) serveValidate(w http.ResponseWriter, r *http.Request) {
	klog.V(4).InfoS("Received validation request", "method", r.Method, "url", r.URL)

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		klog.ErrorS(err, "Failed to read request body")
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	// Verify content type
	if r.Header.Get("Content-Type") != "application/json" {
		klog.ErrorS(nil, "Invalid content type", "contentType", r.Header.Get("Content-Type"))
		http.Error(w, "invalid content type, expected application/json", http.StatusBadRequest)
		return
	}

	// Decode admission review
	admissionReview := admissionv1.AdmissionReview{}
	deserializer := codecs.UniversalDeserializer()
	if _, _, decodeErr := deserializer.Decode(body, nil, &admissionReview); decodeErr != nil {
		klog.ErrorS(decodeErr, "Failed to decode admission review")
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
		klog.ErrorS(err, "Failed to encode response")
		http.Error(w, fmt.Sprintf("failed to encode response: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(responseBytes); err != nil {
		klog.ErrorS(err, "Failed to write response")
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
	klog.InfoS("Validating ZarfPackageJob",
		"name", request.Name,
		"namespace", request.Namespace,
		"operation", request.Operation,
		"user", request.UserInfo.Username)

	// Decode ZarfPackageJob object
	var pkg zarfv1alpha1.ZarfPackageJob
	deserializer := codecs.UniversalDeserializer()
	if _, _, err := deserializer.Decode(request.Object.Raw, nil, &pkg); err != nil {
		klog.ErrorS(err, "Failed to decode ZarfPackageJob")
		return &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: fmt.Sprintf("failed to decode ZarfPackageJob: %v", err),
			},
		}
	}

	// Validate the ZarfPackageJob against ServiceAccount permissions
	if err := ws.zarfValidator.ValidateZarfPackageJob(ctx, &pkg); err != nil {
		klog.InfoS("Validation failed",
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

	klog.InfoS("Validation succeeded",
		"name", pkg.Name,
		"namespace", pkg.Namespace)

	return &admissionv1.AdmissionResponse{
		Allowed: true,
	}
}

// validateUDSBundleJob validates a UDSBundleJob resource
func (ws *WebhookServer) validateUDSBundleJob(ctx context.Context, request *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	klog.InfoS("Validating UDSBundleJob",
		"name", request.Name,
		"namespace", request.Namespace,
		"operation", request.Operation,
		"user", request.UserInfo.Username)

	// Decode UDSBundleJob object
	var bundle udsv1alpha1.UDSBundleJob
	deserializer := codecs.UniversalDeserializer()
	if _, _, err := deserializer.Decode(request.Object.Raw, nil, &bundle); err != nil {
		klog.ErrorS(err, "Failed to decode UDSBundleJob")
		return &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: fmt.Sprintf("failed to decode UDSBundleJob: %v", err),
			},
		}
	}

	// Validate the UDSBundleJob against ServiceAccount permissions
	if err := ws.udsValidator.ValidateUDSBundleJob(ctx, &bundle); err != nil {
		klog.InfoS("Validation failed",
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

	klog.InfoS("Validation succeeded",
		"name", bundle.Name,
		"namespace", bundle.Namespace)

	return &admissionv1.AdmissionResponse{
		Allowed: true,
		Warnings: []string{
			"v1alpha1 UDSBundleJob API is deprecated and will be removed in Forge v0.10.0. Please migrate to v1alpha2 UDSPackageJob. See docs/operations/V1ALPHA2_MIGRATION.md for migration guide.",
		},
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
