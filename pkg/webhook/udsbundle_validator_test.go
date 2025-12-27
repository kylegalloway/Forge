package webhook

import (
	"context"
	"testing"

	udsv1alpha2 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha2"
	"github.com/kylegalloway/forge/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNewUDSPackageJobValidator(t *testing.T) {
	kubeClient := fake.NewClientset()
	validator := NewUDSPackageJobValidator(kubeClient)
	if validator == nil {
		t.Fatal("NewUDSPackageJobValidator returned nil")
	}
	if validator.kubeClient == nil {
		t.Error("kubeClient not set")
	}
}

func TestValidateUDSPackageJob_ValidCreate(t *testing.T) {
	kubeClient := fake.NewClientset()
	validator := NewUDSPackageJobValidator(kubeClient)

	// Create ServiceAccount with permissions
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
			Annotations: map[string]string{
				constants.AnnotationAllowedActions:     "Create,Publish",
				constants.AnnotationAllowedSourceRepos: "https://github.com/test/*",
			},
		},
	}
	_, err := kubeClient.CoreV1().ServiceAccounts("default").Create(context.Background(), sa, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create ServiceAccount: %v", err)
	}

	bundle := &udsv1alpha2.UDSPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-create",
			Namespace: "default",
		},
		Spec: udsv1alpha2.UDSPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             udsv1alpha2.ActionCreate,
			Source: udsv1alpha2.PackageSource{
				Type: udsv1alpha2.SourceTypeGit,
				Git: &udsv1alpha2.GitSource{
					URL: "https://github.com/test/repo",
					Ref: "main",
				},
			},
		},
	}

	err = validator.ValidateUDSPackageJob(context.Background(), bundle)
	if err != nil {
		t.Errorf("ValidateUDSPackageJob() failed for valid bundle: %v", err)
	}
}

func TestValidateUDSPackageJob_MissingServiceAccount(t *testing.T) {
	kubeClient := fake.NewClientset()
	validator := NewUDSPackageJobValidator(kubeClient)

	bundle := &udsv1alpha2.UDSPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-create",
			Namespace: "default",
		},
		Spec: udsv1alpha2.UDSPackageJobSpec{
			ServiceAccountName: "nonexistent-sa",
			Action:             udsv1alpha2.ActionCreate,
			Source: udsv1alpha2.PackageSource{
				Type: udsv1alpha2.SourceTypeGit,
				Git: &udsv1alpha2.GitSource{
					URL: "https://github.com/test/repo",
				},
			},
		},
	}

	err := validator.ValidateUDSPackageJob(context.Background(), bundle)
	if err == nil {
		t.Error("Expected error for missing ServiceAccount, got nil")
	}
}

func TestValidateUDSAction(t *testing.T) {
	validator := &UDSPackageJobValidator{}

	tests := []struct {
		name          string
		annotations   map[string]string
		action        udsv1alpha2.Action
		wantErr       bool
		errorContains string
	}{
		{
			name: "action allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedActions: "Create,Publish",
			},
			action:  udsv1alpha2.ActionCreate,
			wantErr: false,
		},
		{
			name: "action not allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedActions: "Create",
			},
			action:        udsv1alpha2.ActionPublish,
			wantErr:       true,
			errorContains: "not allowed",
		},
		{
			name:          "missing annotation",
			annotations:   map[string]string{},
			action:        udsv1alpha2.ActionCreate,
			wantErr:       true,
			errorContains: "no allowed-actions annotation",
		},
		{
			name: "compound action allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedActions: "Create,Publish,Deploy,CreatePublish",
			},
			action:  udsv1alpha2.ActionCreatePublish,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sa := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-sa",
					Namespace:   "default",
					Annotations: tt.annotations,
				},
			}

			err := validator.validateAction(sa, tt.action)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAction() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errorContains != "" && err != nil {
				if !contains(err.Error(), tt.errorContains) {
					t.Errorf("Error should contain %q, got %q", tt.errorContains, err.Error())
				}
			}
		})
	}
}

func TestValidateUDSPackageSource(t *testing.T) {
	validator := &UDSPackageJobValidator{}

	tests := []struct {
		name          string
		annotations   map[string]string
		source        *udsv1alpha2.PackageSource
		wantErr       bool
		errorContains string
	}{
		{
			name: "git source allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedSourceRepos: "https://github.com/myorg/*",
			},
			source: &udsv1alpha2.PackageSource{
				Type: udsv1alpha2.SourceTypeGit,
				Git: &udsv1alpha2.GitSource{
					URL: "https://github.com/myorg/repo",
				},
			},
			wantErr: false,
		},
		{
			name: "git source not allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedSourceRepos: "https://github.com/myorg/*",
			},
			source: &udsv1alpha2.PackageSource{
				Type: udsv1alpha2.SourceTypeGit,
				Git: &udsv1alpha2.GitSource{
					URL: "https://github.com/otherorg/repo",
				},
			},
			wantErr:       true,
			errorContains: "not allowed",
		},
		{
			name: "OCI source allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedSourceRegistries: "ghcr.io/myorg/*",
			},
			source: &udsv1alpha2.PackageSource{
				Type: udsv1alpha2.SourceTypeOCI,
				OCI: &udsv1alpha2.OCISource{
					Reference: "ghcr.io/myorg/bundle:v1.0.0",
				},
			},
			wantErr: false,
		},
		{
			name: "S3 source allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedSourceBuckets: "my-bucket,other-bucket",
			},
			source: &udsv1alpha2.PackageSource{
				Type: udsv1alpha2.SourceTypeS3,
				S3: &udsv1alpha2.S3Source{
					Bucket: "my-bucket",
					Key:    "bundles/test.tar.zst",
					Region: "us-east-1",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sa := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-sa",
					Namespace:   "default",
					Annotations: tt.annotations,
				},
			}

			err := validator.validateSource(sa, tt.source)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSource() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errorContains != "" && err != nil {
				if !contains(err.Error(), tt.errorContains) {
					t.Errorf("Error should contain %q, got %q", tt.errorContains, err.Error())
				}
			}
		})
	}
}

func TestValidateUDSBundlePublish(t *testing.T) {
	validator := &UDSPackageJobValidator{}

	tests := []struct {
		name          string
		annotations   map[string]string
		publish       *udsv1alpha2.PublishConfig
		wantErr       bool
		errorContains string
	}{
		{
			name: "OCI publish allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedPublishRegistries: "ghcr.io/myorg/*",
			},
			publish: &udsv1alpha2.PublishConfig{
				Destination: udsv1alpha2.PublishDestination{
					Type: udsv1alpha2.DestinationTypeOCI,
					OCI: &udsv1alpha2.OCIDestination{
						Registry:   "ghcr.io",
						Repository: "myorg/bundles",
						Tag:        "v1.0.0",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "OCI publish not allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedPublishRegistries: "ghcr.io/myorg/*",
			},
			publish: &udsv1alpha2.PublishConfig{
				Destination: udsv1alpha2.PublishDestination{
					Type: udsv1alpha2.DestinationTypeOCI,
					OCI: &udsv1alpha2.OCIDestination{
						Registry:   "docker.io",
						Repository: "otherorg/bundles",
					},
				},
			},
			wantErr:       true,
			errorContains: "not allowed",
		},
		{
			name: "S3 publish allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedPublishBuckets: "publish-bucket,backup-bucket",
			},
			publish: &udsv1alpha2.PublishConfig{
				Destination: udsv1alpha2.PublishDestination{
					Type: udsv1alpha2.DestinationTypeS3,
					S3: &udsv1alpha2.S3Destination{
						Bucket: "publish-bucket",
						Key:    "prod/",
						Region: "us-east-1",
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sa := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-sa",
					Namespace:   "default",
					Annotations: tt.annotations,
				},
			}

			err := validator.validatePublish(sa, tt.publish)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePublish() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errorContains != "" && err != nil {
				if !contains(err.Error(), tt.errorContains) {
					t.Errorf("Error should contain %q, got %q", tt.errorContains, err.Error())
				}
			}
		})
	}
}

func TestValidateUDSBundleDeploy(t *testing.T) {
	validator := &UDSPackageJobValidator{}

	tests := []struct {
		name          string
		annotations   map[string]string
		deploy        *udsv1alpha2.DeployConfig
		wantErr       bool
		errorContains string
	}{
		{
			name: "in-cluster deploy allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedDeployTargets: "InCluster",
			},
			deploy: &udsv1alpha2.DeployConfig{
				Target:    udsv1alpha2.DeployTargetInCluster,
				Namespace: "default",
			},
			wantErr: false,
		},
		{
			name: "deploy target not allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedDeployTargets: "InCluster",
			},
			deploy: &udsv1alpha2.DeployConfig{
				Target: udsv1alpha2.DeployTargetExternalCluster,
			},
			wantErr:       true,
			errorContains: "not allowed",
		},
		{
			name:        "missing annotation",
			annotations: map[string]string{},
			deploy: &udsv1alpha2.DeployConfig{
				Target: udsv1alpha2.DeployTargetInCluster,
			},
			wantErr:       true,
			errorContains: "no allowed-deploy-targets annotation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sa := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-sa",
					Namespace:   "default",
					Annotations: tt.annotations,
				},
			}

			err := validator.validateDeploy(sa, tt.deploy)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateDeploy() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errorContains != "" && err != nil {
				if !contains(err.Error(), tt.errorContains) {
					t.Errorf("Error should contain %q, got %q", tt.errorContains, err.Error())
				}
			}
		})
	}
}

func TestValidateUDSBundlePublish_LocalDestination(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
		},
	}

	client := fake.NewClientset(sa)
	validator := NewUDSPackageJobValidator(client)

	publish := &udsv1alpha2.PublishConfig{
		Destination: udsv1alpha2.PublishDestination{
			Type: udsv1alpha2.DestinationTypeLocal,
		},
	}

	err := validator.validatePublish(sa, publish)
	if err != nil {
		t.Errorf("validatePublish() with local destination failed: %v", err)
	}
}

func TestValidateUDSBundlePublish_UnknownDestination(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
		},
	}

	client := fake.NewClientset(sa)
	validator := NewUDSPackageJobValidator(client)

	publish := &udsv1alpha2.PublishConfig{
		Destination: udsv1alpha2.PublishDestination{
			Type: "UnknownType",
		},
	}

	err := validator.validatePublish(sa, publish)
	if err == nil {
		t.Error("validatePublish() with unknown destination should fail")
	}
	if !contains(err.Error(), "unknown publish destination type") {
		t.Errorf("Expected error about unknown type, got: %v", err)
	}
}

func TestValidateUDSPackageSource_UnknownType(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
		},
	}

	client := fake.NewClientset(sa)
	validator := NewUDSPackageJobValidator(client)

	source := &udsv1alpha2.PackageSource{
		Type: "UnknownSourceType",
	}

	err := validator.validateSource(sa, source)
	if err == nil {
		t.Error("validateSource() with unknown type should fail")
	}
	if !contains(err.Error(), "unknown source type") {
		t.Errorf("Expected error about unknown source type, got: %v", err)
	}
}

func TestValidateUDSPackageJob_CompleteWorkflow(t *testing.T) {
	kubeClient := fake.NewClientset()
	validator := NewUDSPackageJobValidator(kubeClient)

	// Create ServiceAccount with full permissions
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
			Annotations: map[string]string{
				constants.AnnotationAllowedActions:           "CreatePublishDeploy",
				constants.AnnotationAllowedSourceRepos:       "https://github.com/test/*",
				constants.AnnotationAllowedPublishRegistries: "ghcr.io/test/*",
				constants.AnnotationAllowedDeployTargets:     "InCluster",
			},
		},
	}
	_, err := kubeClient.CoreV1().ServiceAccounts("default").Create(context.Background(), sa, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create ServiceAccount: %v", err)
	}

	bundle := &udsv1alpha2.UDSPackageJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-complete",
			Namespace: "default",
		},
		Spec: udsv1alpha2.UDSPackageJobSpec{
			ServiceAccountName: "test-sa",
			Action:             udsv1alpha2.ActionCreatePublishDeploy,
			Source: udsv1alpha2.PackageSource{
				Type: udsv1alpha2.SourceTypeGit,
				Git: &udsv1alpha2.GitSource{
					URL: "https://github.com/test/repo",
					Ref: "main",
				},
			},
			Publish: &udsv1alpha2.PublishConfig{
				Destination: udsv1alpha2.PublishDestination{
					Type: udsv1alpha2.DestinationTypeOCI,
					OCI: &udsv1alpha2.OCIDestination{
						Registry:   "ghcr.io",
						Repository: "test/bundles",
						Tag:        "v1.0.0",
					},
				},
			},
			Deploy: &udsv1alpha2.DeployConfig{
				Target:    udsv1alpha2.DeployTargetInCluster,
				Namespace: "default",
			},
		},
	}

	err = validator.ValidateUDSPackageJob(context.Background(), bundle)
	if err != nil {
		t.Errorf("ValidateUDSPackageJob() failed for complete workflow: %v", err)
	}
}
