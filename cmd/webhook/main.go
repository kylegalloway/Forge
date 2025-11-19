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
	"k8s.io/klog/v2"

	scriptrunnerv1alpha1 "github.com/kylegalloway/scriptrunner/pkg/apis/scriptrunner/v1alpha1"
	"github.com/kylegalloway/scriptrunner/pkg/webhook"
)

var (
	certFile   string
	keyFile    string
	port       int
	configFile string
	scheme     = runtime.NewScheme()
	codecs     = serializer.NewCodecFactory(scheme)
)

func init() {
	_ = admissionv1.AddToScheme(scheme)
	_ = scriptrunnerv1alpha1.AddToScheme(scheme)
}

func main() {
	klog.InitFlags(nil)
	flag.StringVar(&certFile, "tls-cert-file", "/etc/webhook/certs/tls.crt", "TLS certificate file")
	flag.StringVar(&keyFile, "tls-key-file", "/etc/webhook/certs/tls.key", "TLS key file")
	flag.IntVar(&port, "port", 8443, "Webhook server port")
	flag.StringVar(&configFile, "config", "/etc/webhook/config/config.json", "Webhook configuration file")
	flag.Parse()

	// Load configuration
	config, err := loadConfig(configFile)
	if err != nil {
		klog.Fatalf("Failed to load config: %v", err)
	}

	klog.InfoS("Webhook configuration loaded",
		"approvedScripts", config.ApprovedScripts,
		"approvedImagePrefix", config.ApprovedImagePrefix,
		"allowInlineScripts", config.AllowInlineScripts)

	// Create validator
	validator := webhook.NewValidator(config)

	// Create webhook server
	server := &WebhookServer{
		validator: validator,
	}

	// Set up HTTP handlers
	mux := http.NewServeMux()
	mux.HandleFunc("/validate", server.serveValidate)
	mux.HandleFunc("/mutate", server.serveMutate)
	mux.HandleFunc("/healthz", server.serveHealthz)
	mux.HandleFunc("/readyz", server.serveReadyz)

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
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
	klog.InfoS("Starting webhook server", "port", port)
	if err := httpServer.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
		klog.Fatalf("Failed to start webhook server: %v", err)
	}

	klog.Info("Webhook server stopped")
}

// WebhookServer handles admission webhook requests
type WebhookServer struct {
	validator *webhook.Validator
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
	if _, _, err := deserializer.Decode(body, nil, &admissionReview); err != nil {
		klog.ErrorS(err, "Failed to decode admission review")
		http.Error(w, fmt.Sprintf("failed to decode admission review: %v", err), http.StatusBadRequest)
		return
	}

	// Process admission request
	response := ws.validate(admissionReview.Request)

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
func (ws *WebhookServer) validate(request *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	klog.InfoS("Validating ScriptRunner",
		"name", request.Name,
		"namespace", request.Namespace,
		"operation", request.Operation,
		"user", request.UserInfo.Username)

	// Decode ScriptRunner object
	var scriptRunner scriptrunnerv1alpha1.ScriptRunner
	deserializer := codecs.UniversalDeserializer()
	if _, _, err := deserializer.Decode(request.Object.Raw, nil, &scriptRunner); err != nil {
		klog.ErrorS(err, "Failed to decode ScriptRunner")
		return &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: fmt.Sprintf("failed to decode ScriptRunner: %v", err),
			},
		}
	}

	// Validate the ScriptRunner
	if err := ws.validator.ValidateScriptRunner(&scriptRunner); err != nil {
		klog.InfoS("Validation failed",
			"name", scriptRunner.Name,
			"namespace", scriptRunner.Namespace,
			"error", err.Error())

		return &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: fmt.Sprintf("ScriptRunner validation failed: %v", err),
			},
		}
	}

	klog.InfoS("Validation succeeded",
		"name", scriptRunner.Name,
		"namespace", scriptRunner.Namespace)

	return &admissionv1.AdmissionResponse{
		Allowed: true,
	}
}

// serveMutate handles mutating webhook requests
func (ws *WebhookServer) serveMutate(w http.ResponseWriter, r *http.Request) {
	klog.V(4).InfoS("Received mutation request", "method", r.Method, "url", r.URL)

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		klog.ErrorS(err, "Failed to read request body")
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	// Decode admission review
	admissionReview := admissionv1.AdmissionReview{}
	deserializer := codecs.UniversalDeserializer()
	if _, _, err := deserializer.Decode(body, nil, &admissionReview); err != nil {
		klog.ErrorS(err, "Failed to decode admission review")
		http.Error(w, fmt.Sprintf("failed to decode admission review: %v", err), http.StatusBadRequest)
		return
	}

	// Process mutation request
	response := ws.mutate(admissionReview.Request)

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

// mutate applies default values and returns JSON patch
func (ws *WebhookServer) mutate(request *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	klog.InfoS("Mutating ScriptRunner",
		"name", request.Name,
		"namespace", request.Namespace)

	// Decode ScriptRunner object
	var scriptRunner scriptrunnerv1alpha1.ScriptRunner
	deserializer := codecs.UniversalDeserializer()
	if _, _, err := deserializer.Decode(request.Object.Raw, nil, &scriptRunner); err != nil {
		klog.ErrorS(err, "Failed to decode ScriptRunner")
		return &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: fmt.Sprintf("failed to decode ScriptRunner: %v", err),
			},
		}
	}

	// Apply defaults
	ws.validator.SetDefaults(&scriptRunner)

	// Marshal the modified object
	modifiedBytes, err := json.Marshal(scriptRunner)
	if err != nil {
		klog.ErrorS(err, "Failed to marshal modified ScriptRunner")
		return &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: fmt.Sprintf("failed to marshal ScriptRunner: %v", err),
			},
		}
	}

	// Create JSON patch
	patch := []byte(fmt.Sprintf(`[{"op":"replace","path":"/spec","value":%s},{"op":"replace","path":"/metadata/labels","value":%s}]`,
		string(modifiedBytes),
		marshalLabels(scriptRunner.Labels)))

	patchType := admissionv1.PatchTypeJSONPatch
	return &admissionv1.AdmissionResponse{
		Allowed:   true,
		PatchType: &patchType,
		Patch:     patch,
	}
}

// serveHealthz handles health check requests
func (ws *WebhookServer) serveHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// serveReadyz handles readiness check requests
func (ws *WebhookServer) serveReadyz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ready"))
}

// loadConfig loads the webhook configuration from a file
func loadConfig(filename string) (webhook.Config, error) {
	var config webhook.Config

	// Set defaults
	config.MaxInputs = 20
	config.MaxInputValueLength = 1000
	config.MaxScriptArgs = 10
	config.MaxScriptArgLength = 200
	config.AllowInlineScripts = false

	// If config file doesn't exist, use defaults
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		klog.InfoS("Config file not found, using defaults", "file", filename)
		return config, nil
	}

	// Read config file
	data, err := os.ReadFile(filename)
	if err != nil {
		return config, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse JSON
	if err := json.Unmarshal(data, &config); err != nil {
		return config, fmt.Errorf("failed to parse config file: %w", err)
	}

	return config, nil
}

// marshalLabels converts labels map to JSON string
func marshalLabels(labels map[string]string) string {
	if labels == nil {
		return "{}"
	}
	bytes, _ := json.Marshal(labels)
	return string(bytes)
}
