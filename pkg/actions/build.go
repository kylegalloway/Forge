// Package actions provides handlers for ZarfPackage actions (Build, Publish, Deploy).
//
// Each action handler is responsible for executing a specific operation
// on a Zarf package or UDS bundle.
package actions

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	zarfv1alpha1 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha1"
	"github.com/kylegalloway/forge/pkg/telemetry"
)

const (
	// ZarfCLIImage is the default Zarf CLI container image
	ZarfCLIImage = "ghcr.io/defenseunicorns/zarf:v0.32.0"
)

// BuildHandler handles Build actions for ZarfPackage resources
type BuildHandler struct {
	kubeClient kubernetes.Interface
	metrics    *telemetry.Metrics
	tracer     *telemetry.Tracer
}

// NewBuildHandler creates a new BuildHandler
func NewBuildHandler(kubeClient kubernetes.Interface, metrics *telemetry.Metrics, tracer *telemetry.Tracer) *BuildHandler {
	return &BuildHandler{
		kubeClient: kubeClient,
		metrics:    metrics,
		tracer:     tracer,
	}
}

// Execute performs a Build action for the given ZarfPackage
func (h *BuildHandler) Execute(ctx context.Context, pkg *zarfv1alpha1.ZarfPackage) (*ActionResult, error) {


	klog.InfoS("Executing Build action", "name", pkg.Name, "namespace", pkg.Namespace)

	// Validate source is provided
	if pkg.Spec.Source.Type == "" {
		return nil, fmt.Errorf("source type is required for Build action")
	}

	// Create Kubernetes Job to build the package
	job, err := h.createBuildJob(ctx, pkg)
	if err != nil {
		return nil, fmt.Errorf("failed to create build job: %w", err)
	}

	klog.InfoS("Build job created", "name", pkg.Name, "job", job.Name)

	result := &ActionResult{
		JobName:    job.Name,
		Phase:      "Running",
		Message:    fmt.Sprintf("Build job %s created", job.Name),
		StartTime:  metav1.Now(),
		Completed:  false,
	}

	return result, nil
}

// createBuildJob creates a Kubernetes Job to build a Zarf package
func (h *BuildHandler) createBuildJob(ctx context.Context, pkg *zarfv1alpha1.ZarfPackage) (*batchv1.Job, error) {
	jobName := fmt.Sprintf("%s-build", pkg.Name)
	namespace := pkg.Namespace

	// Build zarf command based on source type
	zarfCmd, workingDir, err := h.buildZarfCommand(pkg)
	if err != nil {
		return nil, err
	}

	// Build initContainers based on source type (for Git clone, S3 download, etc.)
	initContainers := h.buildInitContainers(pkg)

	// Job configuration
	backoffLimit := int32(0) // Don't retry failed builds
	activeDeadlineSeconds := int64(3600) // 1 hour timeout

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":                          "forge",
				"forge.zarf.dev/package":       pkg.Name,
				"forge.zarf.dev/action":        "build",
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(pkg, zarfv1alpha1.SchemeGroupVersion.WithKind("ZarfPackage")),
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:          &backoffLimit,
			ActiveDeadlineSeconds: &activeDeadlineSeconds,
			TTLSecondsAfterFinished: ptr(int32(3600)), // Clean up after 1 hour
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":                    "forge",
						"forge.zarf.dev/package": pkg.Name,
						"forge.zarf.dev/action":  "build",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy:  corev1.RestartPolicyNever,
					InitContainers: initContainers,
					Containers: []corev1.Container{
						{
							Name:       "zarf-build",
							Image:      ZarfCLIImage,
							Command:    []string{"/bin/sh", "-c"},
							Args:       []string{zarfCmd},
							WorkingDir: workingDir,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "workspace",
									MountPath: "/workspace",
								},
								{
									Name:      "output",
									MountPath: "/output",
								},
							},
							SecurityContext: &corev1.SecurityContext{
								RunAsNonRoot:             ptr(true),
								RunAsUser:                ptr(int64(1000)),
								AllowPrivilegeEscalation: ptr(false),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
								SeccompProfile: &corev1.SeccompProfile{
									Type: corev1.SeccompProfileTypeRuntimeDefault,
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    mustParseQuantity("500m"),
									corev1.ResourceMemory: mustParseQuantity("1Gi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    mustParseQuantity("2000m"),
									corev1.ResourceMemory: mustParseQuantity("4Gi"),
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "workspace",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: "output",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: ptr(true),
						RunAsUser:    ptr(int64(1000)),
						FSGroup:      ptr(int64(1000)),
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
				},
			},
		},
	}

	// Add docker-config volume if OCI source with credentials
	if pkg.Spec.Source.Type == zarfv1alpha1.SourceTypeOCI && pkg.Spec.Source.OCI != nil && pkg.Spec.Source.OCI.CredentialsSecretRef != nil {
		job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: "docker-config",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: pkg.Spec.Source.OCI.CredentialsSecretRef.Name,
					Items: []corev1.KeyToPath{
						{
							Key:  ".dockerconfigjson",
							Path: "config.json",
						},
					},
				},
			},
		})
	}

	// Create the job
	createdJob, err := h.kubeClient.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	return createdJob, nil
}

// buildZarfCommand builds the zarf CLI command based on package source
func (h *BuildHandler) buildZarfCommand(pkg *zarfv1alpha1.ZarfPackage) (string, string, error) {
	workingDir := "/workspace"

	// Basic zarf package create command
	cmd := "zarf package create . --confirm --output-directory /output"

	return cmd, workingDir, nil
}

// buildInitContainers creates init containers for source retrieval
func (h *BuildHandler) buildInitContainers(pkg *zarfv1alpha1.ZarfPackage) []corev1.Container {
	var initContainers []corev1.Container

	switch pkg.Spec.Source.Type {
	case zarfv1alpha1.SourceTypeGit:
		// Git clone init container
		gitSource := pkg.Spec.Source.Git
		if gitSource == nil {
			break
		}

		cloneCmd := fmt.Sprintf("git clone --depth 1 --branch %s %s /workspace", gitSource.Ref, gitSource.URL)
		if gitSource.Path != "" && gitSource.Path != "." {
			cloneCmd = fmt.Sprintf("%s && cd /workspace && mv %s/* . && rm -rf %s", cloneCmd, gitSource.Path, gitSource.Path)
		}

		initContainers = append(initContainers, corev1.Container{
			Name:    "git-clone",
			Image:   "alpine/git:latest",
			Command: []string{"/bin/sh", "-c"},
			Args:    []string{cloneCmd},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "workspace",
					MountPath: "/workspace",
				},
			},
			SecurityContext: &corev1.SecurityContext{
				RunAsNonRoot:             ptr(true),
				RunAsUser:                ptr(int64(1000)),
				AllowPrivilegeEscalation: ptr(false),
				Capabilities: &corev1.Capabilities{
					Drop: []corev1.Capability{"ALL"},
				},
			},
		})

	case zarfv1alpha1.SourceTypeS3:
		// S3 download init container
		s3Source := pkg.Spec.Source.S3
		if s3Source == nil {
			break
		}

		// Build S3 download command
		s3Path := fmt.Sprintf("s3://%s/%s", s3Source.Bucket, s3Source.Key)
		downloadCmd := fmt.Sprintf("aws s3 cp %s /workspace/package.tar.zst --region %s", s3Path, s3Source.Region)

		// Build environment variables for credentials
		env := []corev1.EnvVar{}
		if s3Source.CredentialsSecretRef != nil {
			env = append(env,
				corev1.EnvVar{
					Name: "AWS_ACCESS_KEY_ID",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: s3Source.CredentialsSecretRef.Name,
							},
							Key: "access-key-id",
						},
					},
				},
				corev1.EnvVar{
					Name: "AWS_SECRET_ACCESS_KEY",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: s3Source.CredentialsSecretRef.Name,
							},
							Key: "secret-access-key",
						},
					},
				},
			)
		}

		initContainers = append(initContainers, corev1.Container{
			Name:    "s3-download",
			Image:   "amazon/aws-cli:latest",
			Command: []string{"/bin/sh", "-c"},
			Args:    []string{downloadCmd},
			Env:     env,
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "workspace",
					MountPath: "/workspace",
				},
			},
			SecurityContext: &corev1.SecurityContext{
				RunAsNonRoot:             ptr(true),
				RunAsUser:                ptr(int64(1000)),
				AllowPrivilegeEscalation: ptr(false),
				Capabilities: &corev1.Capabilities{
					Drop: []corev1.Capability{"ALL"},
				},
			},
		})

	case zarfv1alpha1.SourceTypeOCI:
		// OCI pull init container
		ociSource := pkg.Spec.Source.OCI
		if ociSource == nil {
			break
		}

		// Use crane (go-containerregistry) to pull OCI artifacts
		// crane export <image> - | tar -xz -C /workspace
		pullCmd := fmt.Sprintf("crane export %s - | tar -xz -C /workspace", ociSource.Image)

		// Build volume mounts
		volumeMounts := []corev1.VolumeMount{
			{
				Name:      "workspace",
				MountPath: "/workspace",
			},
		}

		// Add docker config volume mount if credentials provided
		if ociSource.CredentialsSecretRef != nil {
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      "docker-config",
				MountPath: "/home/nonroot/.docker",
				ReadOnly:  true,
			})
		}

		initContainers = append(initContainers, corev1.Container{
			Name:         "oci-pull",
			Image:        "gcr.io/go-containerregistry/crane:latest",
			Command:      []string{"/bin/sh", "-c"},
			Args:         []string{pullCmd},
			VolumeMounts: volumeMounts,
			SecurityContext: &corev1.SecurityContext{
				RunAsNonRoot:             ptr(true),
				RunAsUser:                ptr(int64(65532)), // nonroot user in crane image
				AllowPrivilegeEscalation: ptr(false),
				Capabilities: &corev1.Capabilities{
					Drop: []corev1.Capability{"ALL"},
				},
			},
		})

	case zarfv1alpha1.SourceTypeLocal:
		// Local source - no init container needed, but this shouldn't be used in production
		klog.V(4).InfoS("Local source - no init container needed", "package", pkg.Name)
	}

	return initContainers
}
