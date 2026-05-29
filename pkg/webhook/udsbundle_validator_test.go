package webhook

import (
	"context"
	"testing"

	udsv1alpha3 "github.com/kylegalloway/forge/pkg/apis/uds/v1alpha3"
	"github.com/kylegalloway/forge/pkg/audit"
	"github.com/kylegalloway/forge/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNewUDSBundleJobValidator(t *testing.T) {
	kubeClient := fake.NewClientset()
	validator := NewUDSBundleJobValidator(kubeClient, &audit.NoopAuditTrail{})
	if validator == nil {
		t.Fatal("NewUDSBundleJobValidator returned nil")
	}
	if validator.pv == nil {
		t.Error("permission validator not set")
	}
}

func TestValidateUDSBundleJob_ValidCreate(t *testing.T) {
	kubeClient := fake.NewClientset()
	validator := NewUDSBundleJobValidator(kubeClient, &audit.NoopAuditTrail{})

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

	bundle := &udsv1alpha3.UDSBundleJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-create",
			Namespace: "default",
		},
		Spec: udsv1alpha3.UDSBundleJobSpec{
			ServiceAccountName: "test-sa",
			Action:             udsv1alpha3.ActionCreate,
			Source: udsv1alpha3.PackageSource{
				Type: udsv1alpha3.SourceTypeGit,
				Git: &udsv1alpha3.GitSource{
					URL: "https://github.com/test/repo",
					Ref: "main",
				},
			},
		},
	}

	err = validator.ValidateUDSBundleJob(context.Background(), bundle)
	if err != nil {
		t.Errorf("ValidateUDSBundleJob() failed for valid bundle: %v", err)
	}
}

func TestValidateUDSBundleJob_MissingServiceAccount(t *testing.T) {
	kubeClient := fake.NewClientset()
	validator := NewUDSBundleJobValidator(kubeClient, &audit.NoopAuditTrail{})

	bundle := &udsv1alpha3.UDSBundleJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-create",
			Namespace: "default",
		},
		Spec: udsv1alpha3.UDSBundleJobSpec{
			ServiceAccountName: "nonexistent-sa",
			Action:             udsv1alpha3.ActionCreate,
			Source: udsv1alpha3.PackageSource{
				Type: udsv1alpha3.SourceTypeGit,
				Git: &udsv1alpha3.GitSource{
					URL: "https://github.com/test/repo",
				},
			},
		},
	}

	err := validator.ValidateUDSBundleJob(context.Background(), bundle)
	if err == nil {
		t.Error("Expected error for missing ServiceAccount, got nil")
	}
}

func TestValidateUDSAction(t *testing.T) {
	validator := &UDSBundleJobValidator{}

	tests := []struct {
		name          string
		annotations   map[string]string
		action        udsv1alpha3.Action
		wantErr       bool
		errorContains string
	}{
		{
			name: "action allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedActions: "Create,Publish",
			},
			action:  udsv1alpha3.ActionCreate,
			wantErr: false,
		},
		{
			name: "action not allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedActions: "Create",
			},
			action:        udsv1alpha3.ActionPublish,
			wantErr:       true,
			errorContains: "not allowed",
		},
		{
			name:          "missing annotation",
			annotations:   map[string]string{},
			action:        udsv1alpha3.ActionCreate,
			wantErr:       true,
			errorContains: "no forge.dev/allowed-actions annotation",
		},
		{
			name: "compound action allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedActions: "Create,Publish,Deploy,CreatePublish",
			},
			action:  udsv1alpha3.ActionCreatePublish,
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
	validator := &UDSBundleJobValidator{}

	tests := []struct {
		name          string
		annotations   map[string]string
		source        *udsv1alpha3.PackageSource
		wantErr       bool
		errorContains string
	}{
		{
			name: "git source allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedSourceRepos: "https://github.com/myorg/*",
			},
			source: &udsv1alpha3.PackageSource{
				Type: udsv1alpha3.SourceTypeGit,
				Git: &udsv1alpha3.GitSource{
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
			source: &udsv1alpha3.PackageSource{
				Type: udsv1alpha3.SourceTypeGit,
				Git: &udsv1alpha3.GitSource{
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
			source: &udsv1alpha3.PackageSource{
				Type: udsv1alpha3.SourceTypeOCI,
				OCI: &udsv1alpha3.OCISource{
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
			source: &udsv1alpha3.PackageSource{
				Type: udsv1alpha3.SourceTypeS3,
				S3: &udsv1alpha3.S3Source{
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
	validator := &UDSBundleJobValidator{}

	tests := []struct {
		name          string
		annotations   map[string]string
		publish       *udsv1alpha3.PublishConfig
		wantErr       bool
		errorContains string
	}{
		{
			name: "OCI publish allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedPublishRegistries: "ghcr.io/myorg/*",
			},
			publish: &udsv1alpha3.PublishConfig{
				Destination: udsv1alpha3.PublishDestination{
					Type: udsv1alpha3.DestinationTypeOCI,
					OCI: &udsv1alpha3.OCIDestination{
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
			publish: &udsv1alpha3.PublishConfig{
				Destination: udsv1alpha3.PublishDestination{
					Type: udsv1alpha3.DestinationTypeOCI,
					OCI: &udsv1alpha3.OCIDestination{
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
			publish: &udsv1alpha3.PublishConfig{
				Destination: udsv1alpha3.PublishDestination{
					Type: udsv1alpha3.DestinationTypeS3,
					S3: &udsv1alpha3.S3Destination{
						Bucket:    "publish-bucket",
						KeyPrefix: "prod/",
						Region:    "us-east-1",
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
	validator := &UDSBundleJobValidator{}

	tests := []struct {
		name          string
		annotations   map[string]string
		deploy        *udsv1alpha3.DeployConfig
		wantErr       bool
		errorContains string
	}{
		{
			name: "in-cluster deploy allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedDeployTargets: "InCluster",
			},
			deploy: &udsv1alpha3.DeployConfig{
				Target:    udsv1alpha3.DeployTargetInCluster,
				Namespace: "default",
			},
			wantErr: false,
		},
		{
			name: "deploy target not allowed",
			annotations: map[string]string{
				constants.AnnotationAllowedDeployTargets: "InCluster",
			},
			deploy: &udsv1alpha3.DeployConfig{
				Target: udsv1alpha3.DeployTargetExternalCluster,
			},
			wantErr:       true,
			errorContains: "not allowed",
		},
		{
			name:        "missing annotation",
			annotations: map[string]string{},
			deploy: &udsv1alpha3.DeployConfig{
				Target: udsv1alpha3.DeployTargetInCluster,
			},
			wantErr:       true,
			errorContains: "no forge.dev/allowed-deploy-targets annotation",
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
	validator := NewUDSBundleJobValidator(client, &audit.NoopAuditTrail{})

	publish := &udsv1alpha3.PublishConfig{
		Destination: udsv1alpha3.PublishDestination{
			Type: udsv1alpha3.DestinationTypeLocal,
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
	validator := NewUDSBundleJobValidator(client, &audit.NoopAuditTrail{})

	publish := &udsv1alpha3.PublishConfig{
		Destination: udsv1alpha3.PublishDestination{
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
	validator := NewUDSBundleJobValidator(client, &audit.NoopAuditTrail{})

	source := &udsv1alpha3.PackageSource{
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

func TestValidateExtraArgs_PreTasks(t *testing.T) {
	kubeClient := fake.NewClientset()
	validator := NewUDSBundleJobValidator(kubeClient, &audit.NoopAuditTrail{})

	// Create ServiceAccount with permissions
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa-pretasks",
			Namespace: "default",
			Annotations: map[string]string{
				constants.AnnotationAllowedActions:       "Create,Deploy",
				constants.AnnotationAllowedSourceRepos:   "https://github.com/test/*",
				constants.AnnotationAllowedDeployTargets: "InCluster",
			},
		},
	}
	_, err := kubeClient.CoreV1().ServiceAccounts("default").Create(context.Background(), sa, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create ServiceAccount: %v", err)
	}

	tests := []struct {
		name    string
		bundle  *udsv1alpha3.UDSBundleJob
		wantErr bool
	}{
		{
			name: "valid create with pre-tasks",
			bundle: &udsv1alpha3.UDSBundleJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pretask-valid",
					Namespace: "default",
				},
				Spec: udsv1alpha3.UDSBundleJobSpec{
					ServiceAccountName: "test-sa-pretasks",
					Action:             udsv1alpha3.ActionCreate,
					Source: udsv1alpha3.PackageSource{
						Type: udsv1alpha3.SourceTypeGit,
						Git:  &udsv1alpha3.GitSource{URL: "https://github.com/test/repo", Ref: "main"},
					},
					Create: &udsv1alpha3.CreateConfig{
						PreTasks: []udsv1alpha3.RunnerPreTask{
							{Name: "setup-deps", Variables: map[string]string{"VERSION": "1.0"}},
							{Name: "generate-config"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "create with injection in pre-task name",
			bundle: &udsv1alpha3.UDSBundleJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pretask-injection",
					Namespace: "default",
				},
				Spec: udsv1alpha3.UDSBundleJobSpec{
					ServiceAccountName: "test-sa-pretasks",
					Action:             udsv1alpha3.ActionCreate,
					Source: udsv1alpha3.PackageSource{
						Type: udsv1alpha3.SourceTypeGit,
						Git:  &udsv1alpha3.GitSource{URL: "https://github.com/test/repo", Ref: "main"},
					},
					Create: &udsv1alpha3.CreateConfig{
						PreTasks: []udsv1alpha3.RunnerPreTask{
							{Name: "task; rm -rf /"},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "deploy with valid pre-tasks",
			bundle: &udsv1alpha3.UDSBundleJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pretask-deploy",
					Namespace: "default",
				},
				Spec: udsv1alpha3.UDSBundleJobSpec{
					ServiceAccountName: "test-sa-pretasks",
					Action:             udsv1alpha3.ActionDeploy,
					Source: udsv1alpha3.PackageSource{
						Type: udsv1alpha3.SourceTypeGit,
						Git:  &udsv1alpha3.GitSource{URL: "https://github.com/test/repo", Ref: "main"},
					},
					Deploy: &udsv1alpha3.DeployConfig{
						Target: udsv1alpha3.DeployTargetInCluster,
						PreTasks: []udsv1alpha3.RunnerPreTask{
							{Name: "pre-deploy-check"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "deploy with injection in pre-task variable",
			bundle: &udsv1alpha3.UDSBundleJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pretask-deploy-injection",
					Namespace: "default",
				},
				Spec: udsv1alpha3.UDSBundleJobSpec{
					ServiceAccountName: "test-sa-pretasks",
					Action:             udsv1alpha3.ActionDeploy,
					Source: udsv1alpha3.PackageSource{
						Type: udsv1alpha3.SourceTypeGit,
						Git:  &udsv1alpha3.GitSource{URL: "https://github.com/test/repo", Ref: "main"},
					},
					Deploy: &udsv1alpha3.DeployConfig{
						Target: udsv1alpha3.DeployTargetInCluster,
						PreTasks: []udsv1alpha3.RunnerPreTask{
							{Name: "task", Variables: map[string]string{"KEY": "val$(whoami)"}},
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateUDSBundleJob(context.Background(), tt.bundle)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateUDSBundleJob() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateUDSBundleJob_CompleteWorkflow(t *testing.T) {
	kubeClient := fake.NewClientset()
	validator := NewUDSBundleJobValidator(kubeClient, &audit.NoopAuditTrail{})

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

	bundle := &udsv1alpha3.UDSBundleJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-complete",
			Namespace: "default",
		},
		Spec: udsv1alpha3.UDSBundleJobSpec{
			ServiceAccountName: "test-sa",
			Action:             udsv1alpha3.ActionCreatePublishDeploy,
			Source: udsv1alpha3.PackageSource{
				Type: udsv1alpha3.SourceTypeGit,
				Git: &udsv1alpha3.GitSource{
					URL: "https://github.com/test/repo",
					Ref: "main",
				},
			},
			Publish: &udsv1alpha3.PublishConfig{
				Destination: udsv1alpha3.PublishDestination{
					Type: udsv1alpha3.DestinationTypeOCI,
					OCI: &udsv1alpha3.OCIDestination{
						Registry:   "ghcr.io",
						Repository: "test/bundles",
						Tag:        "v1.0.0",
					},
				},
			},
			Deploy: &udsv1alpha3.DeployConfig{
				Target:    udsv1alpha3.DeployTargetInCluster,
				Namespace: "default",
			},
		},
	}

	err = validator.ValidateUDSBundleJob(context.Background(), bundle)
	if err != nil {
		t.Errorf("ValidateUDSBundleJob() failed for complete workflow: %v", err)
	}
}

// TestValidateAction_CaseSensitivityUDS tests that action validation is case-sensitive for UDS
func TestValidateAction_CaseSensitivityUDS(t *testing.T) {
	validator := &UDSBundleJobValidator{}

	tests := []struct {
		name          string
		allowedAction string
		action        udsv1alpha3.Action
		wantErr       bool
	}{
		{"exact case match", "Create", udsv1alpha3.ActionCreate, false},
		{"wrong action", "Create", udsv1alpha3.ActionPublish, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sa := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sa",
					Namespace: "default",
					Annotations: map[string]string{
						constants.AnnotationAllowedActions: tt.allowedAction,
					},
				},
			}

			err := validator.validateAction(sa, tt.action)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAction() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateSource_MultiplePatterns_UDS tests glob pattern matching with multiple patterns
func TestValidateSource_MultiplePatterns_UDS(t *testing.T) {
	validator := &UDSBundleJobValidator{}

	tests := []struct {
		name           string
		allowedPattern string
		gitURL         string
		wantErr        bool
	}{
		{"matches first pattern", "https://github.com/org1/*,https://gitlab.com/org2/*", "https://github.com/org1/repo", false},
		{"matches second pattern", "https://github.com/org1/*,https://gitlab.com/org2/*", "https://gitlab.com/org2/repo", false},
		{"matches neither pattern", "https://github.com/org1/*,https://gitlab.com/org2/*", "https://github.com/org2/repo", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sa := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sa",
					Namespace: "default",
					Annotations: map[string]string{
						constants.AnnotationAllowedSourceRepos: tt.allowedPattern,
					},
				},
			}

			source := &udsv1alpha3.PackageSource{
				Type: udsv1alpha3.SourceTypeGit,
				Git: &udsv1alpha3.GitSource{
					URL: tt.gitURL,
				},
			}

			err := validator.validateSource(sa, source)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSource() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateAction_AllUDSActions tests all supported UDS actions
func TestValidateAction_AllUDSActions(t *testing.T) {
	validator := &UDSBundleJobValidator{}

	actions := []udsv1alpha3.Action{
		udsv1alpha3.ActionCreate,
		udsv1alpha3.ActionPublish,
		udsv1alpha3.ActionDeploy,
		udsv1alpha3.ActionCreatePublish,
		udsv1alpha3.ActionCreatePublishDeploy,
		udsv1alpha3.ActionPublishDeploy,
	}

	allowedActionsStr := "Create,Publish,Deploy,CreatePublish,CreatePublishDeploy,PublishDeploy"

	for _, action := range actions {
		t.Run(string(action), func(t *testing.T) {
			sa := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sa",
					Namespace: "default",
					Annotations: map[string]string{
						constants.AnnotationAllowedActions: allowedActionsStr,
					},
				},
			}

			err := validator.validateAction(sa, action)
			if err != nil {
				t.Errorf("validateAction() failed for action %s: %v", action, err)
			}
		})
	}
}

// TestValidateGitSource_ReferenceVariations_UDS tests various Git reference formats
func TestValidateGitSource_ReferenceVariations_UDS(t *testing.T) {
	validator := &UDSBundleJobValidator{}

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
			Annotations: map[string]string{
				constants.AnnotationAllowedSourceRepos: "https://github.com/kylegalloway/*",
			},
		},
	}

	tests := []struct {
		name string
		ref  string
	}{
		{"main branch", "main"},
		{"feature branch", "feature/test"},
		{"tag reference", "v1.0.0"},
		{"commit SHA", "abc123def"}, // pragma: allowlist secret
		{"empty ref", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := &udsv1alpha3.PackageSource{
				Type: udsv1alpha3.SourceTypeGit,
				Git: &udsv1alpha3.GitSource{
					URL: "https://github.com/kylegalloway/uds",
					Ref: tt.ref,
				},
			}

			err := validator.validateSource(sa, source)
			if err != nil {
				t.Errorf("validateSource() failed for ref %q: %v", tt.ref, err)
			}
		})
	}
}

// TestValidateOCISource_ImageVariations_UDS tests various OCI image reference formats
func TestValidateOCISource_ImageVariations_UDS(t *testing.T) {
	validator := &UDSBundleJobValidator{}

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "default",
			Annotations: map[string]string{
				constants.AnnotationAllowedSourceRegistries: "ghcr.io/*,docker.io/library/*",
			},
		},
	}

	tests := []struct {
		name      string
		reference string
		wantErr   bool
	}{
		{"ghcr with tag", "ghcr.io/kylegalloway/uds:v1.0.0", false},
		{"ghcr with digest", "ghcr.io/kylegalloway/uds@sha256:abc123", false},
		{"docker library allowed", "docker.io/library/nginx:latest", false},
		{"docker non-library not allowed", "docker.io/myorg/image:latest", true},
		{"no tag", "ghcr.io/kylegalloway/uds", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := &udsv1alpha3.PackageSource{
				Type: udsv1alpha3.SourceTypeOCI,
				OCI: &udsv1alpha3.OCISource{
					Reference: tt.reference,
				},
			}

			err := validator.validateSource(sa, source)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSource() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateDeploy_NamespaceValidation_UDS tests namespace specification for deploy
func TestValidateDeploy_NamespaceValidation_UDS(t *testing.T) {
	validator := &UDSBundleJobValidator{}

	tests := []struct {
		name      string
		target    udsv1alpha3.DeployTargetType
		namespace string
		wantErr   bool
	}{
		{"in-cluster with namespace", udsv1alpha3.DeployTargetInCluster, "default", false},
		{"in-cluster empty namespace", udsv1alpha3.DeployTargetInCluster, "", false},
		{"external cluster", udsv1alpha3.DeployTargetExternalCluster, "default", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sa := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sa",
					Namespace: "default",
					Annotations: map[string]string{
						constants.AnnotationAllowedDeployTargets: "InCluster,ExternalCluster",
					},
				},
			}

			deploy := &udsv1alpha3.DeployConfig{
				Target:    tt.target,
				Namespace: tt.namespace,
			}

			err := validator.validateDeploy(sa, deploy)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateDeploy() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidatePublish_AllDestinationTypes_UDS tests all publish destination types for UDS
func TestValidatePublish_AllDestinationTypes_UDS(t *testing.T) {
	validator := &UDSBundleJobValidator{}

	tests := []struct {
		name        string
		publish     *udsv1alpha3.PublishConfig
		annotations map[string]string
		wantErr     bool
	}{
		{
			name: "OCI destination",
			publish: &udsv1alpha3.PublishConfig{
				Destination: udsv1alpha3.PublishDestination{
					Type: udsv1alpha3.DestinationTypeOCI,
					OCI: &udsv1alpha3.OCIDestination{
						Registry:   "ghcr.io",
						Repository: "myorg/bundles",
					},
				},
			},
			annotations: map[string]string{
				constants.AnnotationAllowedPublishRegistries: "ghcr.io/*",
			},
			wantErr: false,
		},
		{
			name: "S3 destination",
			publish: &udsv1alpha3.PublishConfig{
				Destination: udsv1alpha3.PublishDestination{
					Type: udsv1alpha3.DestinationTypeS3,
					S3: &udsv1alpha3.S3Destination{
						Bucket: "my-bucket",
					},
				},
			},
			annotations: map[string]string{
				constants.AnnotationAllowedPublishBuckets: "my-bucket,backup-bucket",
			},
			wantErr: false,
		},
		{
			name: "Local destination",
			publish: &udsv1alpha3.PublishConfig{
				Destination: udsv1alpha3.PublishDestination{
					Type: udsv1alpha3.DestinationTypeLocal,
					Local: &udsv1alpha3.LocalDestination{
						Path:    "/tmp/bundles",
						DevMode: true,
					},
				},
			},
			annotations: map[string]string{},
			wantErr:     false,
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
		})
	}
}

// TestAnnotationParsingWhitespace_UDS tests annotation parsing with various whitespace patterns
func TestAnnotationParsingWhitespace_UDS(t *testing.T) {
	validator := &UDSBundleJobValidator{}

	tests := []struct {
		name       string
		annotation string
		action     udsv1alpha3.Action
		wantErr    bool
	}{
		{"no spaces", "Create,Publish,Deploy", udsv1alpha3.ActionCreate, false},
		{"spaces after comma", "Create, Publish, Deploy", udsv1alpha3.ActionCreate, false},
		{"mixed spacing", "Create ,Publish , Deploy", udsv1alpha3.ActionCreate, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sa := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sa",
					Namespace: "default",
					Annotations: map[string]string{
						constants.AnnotationAllowedActions: tt.annotation,
					},
				},
			}

			err := validator.validateAction(sa, tt.action)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAction() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateExtraArgs_UDS tests command injection prevention in extra arguments
func TestValidateExtraArgs_UDS(t *testing.T) {
	validator := &UDSBundleJobValidator{}

	tests := []struct {
		name        string
		extraArgs   []string
		wantErr     bool
		errorSubstr string
	}{
		{
			name:      "clean arguments",
			extraArgs: []string{"--registry", "ghcr.io", "--output", "/tmp"},
			wantErr:   false,
		},
		{
			name:        "command injection attempt with semicolon",
			extraArgs:   []string{"--output", "/tmp; rm -rf /"},
			wantErr:     true,
			errorSubstr: "forbidden character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := &udsv1alpha3.UDSBundleJobSpec{
				ServiceAccountName: "test-sa",
				Action:             udsv1alpha3.ActionCreate,
				Source: udsv1alpha3.PackageSource{
					Type: udsv1alpha3.SourceTypeGit,
					Git: &udsv1alpha3.GitSource{
						URL: "https://github.com/test/repo",
					},
				},
			}

			// Add extra args to Create config if present
			if len(tt.extraArgs) > 0 {
				spec.Create = &udsv1alpha3.CreateConfig{
					ExtraArgs: tt.extraArgs,
				}
			}

			err := validator.validateExtraArgs(spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateExtraArgs() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errorSubstr != "" && err != nil {
				if !contains(err.Error(), tt.errorSubstr) {
					t.Errorf("Error should contain %q, got %q", tt.errorSubstr, err.Error())
				}
			}
		})
	}
}

// TestValidateSource_EdgeCases_UDS tests edge cases in glob pattern matching for UDS
func TestValidateSource_EdgeCases_UDS(t *testing.T) {
	validator := &UDSBundleJobValidator{}

	tests := []struct {
		name           string
		allowedPattern string
		gitURL         string
		wantErr        bool
	}{
		{"wildcard allows all", "*", "https://github.com/any/url", false},
		{"URL with port", "https://github.com:443/*", "https://github.com:443/org/repo", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sa := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sa",
					Namespace: "default",
					Annotations: map[string]string{
						constants.AnnotationAllowedSourceRepos: tt.allowedPattern,
					},
				},
			}

			source := &udsv1alpha3.PackageSource{
				Type: udsv1alpha3.SourceTypeGit,
				Git: &udsv1alpha3.GitSource{
					URL: tt.gitURL,
				},
			}

			err := validator.validateSource(sa, source)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSource() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
