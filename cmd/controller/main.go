package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/kylegalloway/scriptrunner/pkg/controller"
)

var (
	masterURL  string
	kubeconfig string
	namespace  string
)

func main() {
	klog.InitFlags(nil)
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&namespace, "namespace", "", "Namespace to watch. Empty string means all namespaces.")
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

	// Create and run the controller
	controller := controller.NewSimpleController(kubeClient, dynamicClient, watchNamespace)

	klog.Info("Starting ScriptRunner controller")
	if err := controller.Run(ctx); err != nil {
		klog.Fatalf("Error running controller: %v", err)
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
