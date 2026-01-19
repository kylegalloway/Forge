package destinations

import (
	"testing"

	"github.com/kylegalloway/forge/pkg/apis/common"
	zarfv1alpha3 "github.com/kylegalloway/forge/pkg/apis/zarf/v1alpha3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		pkg      *zarfv1alpha3.ZarfPackageJob
		wantType string
		wantErr  bool
	}{
		{
			name: "s3 destination",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Publish: &zarfv1alpha3.PublishConfig{
						Destination: zarfv1alpha3.PublishDestination{
							Type: zarfv1alpha3.DestinationTypeS3,
						},
					},
				},
			},
			wantType: "*destinations.S3Destination",
			wantErr:  false,
		},
		{
			name: "oci destination",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Publish: &zarfv1alpha3.PublishConfig{
						Destination: zarfv1alpha3.PublishDestination{
							Type: zarfv1alpha3.DestinationTypeOCI,
						},
					},
				},
			},
			wantType: "*destinations.OCIDestination",
			wantErr:  false,
		},
		{
			name: "local destination",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Publish: &zarfv1alpha3.PublishConfig{
						Destination: zarfv1alpha3.PublishDestination{
							Type: zarfv1alpha3.DestinationTypeLocal,
						},
					},
				},
			},
			wantType: "*destinations.LocalDestination",
			wantErr:  false,
		},
		{
			name: "missing publish config",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				Spec: zarfv1alpha3.ZarfPackageJobSpec{},
			},
			wantErr: true,
		},
		{
			name: "unsupported destination type",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Publish: &zarfv1alpha3.PublishConfig{
						Destination: zarfv1alpha3.PublishDestination{
							Type: "UnsupportedType",
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dest, err := New(tt.pkg)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && dest == nil {
				t.Error("New() returned nil destination")
			}
		})
	}
}

func TestS3DestinationGetPublishCommand(t *testing.T) {
	tests := []struct {
		name         string
		pkg          *zarfv1alpha3.ZarfPackageJob
		artifactPath string
		wantErr      bool
	}{
		{
			name: "basic s3 upload",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pkg",
					Namespace: "default",
				},
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Publish: &zarfv1alpha3.PublishConfig{
						Destination: zarfv1alpha3.PublishDestination{
							Type: zarfv1alpha3.DestinationTypeS3,
							S3: &zarfv1alpha3.S3Destination{
								Bucket:    "my-bucket",
								KeyPrefix: "packages/",
								Region:    "us-east-1",
							},
						},
					},
				},
			},
			artifactPath: "/workspace/test.tar.zst",
			wantErr:      false,
		},
		{
			name: "missing s3 config",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pkg",
					Namespace: "default",
				},
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Publish: &zarfv1alpha3.PublishConfig{
						Destination: zarfv1alpha3.PublishDestination{
							Type: zarfv1alpha3.DestinationTypeS3,
						},
					},
				},
			},
			artifactPath: "/workspace/test.tar.zst",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dest := &S3Destination{}
			cmd, err := dest.GetPublishCommand(tt.pkg, tt.artifactPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetPublishCommand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && cmd == "" {
				t.Error("GetPublishCommand() returned empty command")
			}
		})
	}
}

func TestS3DestinationGetJobConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		pkg     *zarfv1alpha3.ZarfPackageJob
		wantErr bool
	}{
		{
			name: "s3 without credentials",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Publish: &zarfv1alpha3.PublishConfig{
						Destination: zarfv1alpha3.PublishDestination{
							Type: zarfv1alpha3.DestinationTypeS3,
							S3: &zarfv1alpha3.S3Destination{
								Bucket: "my-bucket",
								Region: "us-east-1",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "s3 with credentials",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Publish: &zarfv1alpha3.PublishConfig{
						Destination: zarfv1alpha3.PublishDestination{
							Type: zarfv1alpha3.DestinationTypeS3,
							S3: &zarfv1alpha3.S3Destination{
								Bucket: "my-bucket",
								Region: "us-east-1",
								CredentialRef: &common.SecretReference{
									Name: "aws-creds",
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dest := &S3Destination{}
			config, err := dest.GetJobConfiguration(tt.pkg)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetJobConfiguration() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && config == nil {
				t.Error("GetJobConfiguration() returned nil config")
			}
		})
	}
}

func TestOCIDestinationGetPublishCommand(t *testing.T) {
	tests := []struct {
		name         string
		pkg          *zarfv1alpha3.ZarfPackageJob
		artifactPath string
		wantErr      bool
	}{
		{
			name: "basic oci push",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pkg",
					Namespace: "default",
				},
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Publish: &zarfv1alpha3.PublishConfig{
						Destination: zarfv1alpha3.PublishDestination{
							Type: zarfv1alpha3.DestinationTypeOCI,
							OCI: &zarfv1alpha3.OCIDestination{
								Registry:   "ghcr.io",
								Repository: "test/packages",
								Tag:        "v1.0.0",
							},
						},
					},
				},
			},
			artifactPath: "/workspace/test.tar.zst",
			wantErr:      false,
		},
		{
			name: "missing oci config",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pkg",
					Namespace: "default",
				},
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Publish: &zarfv1alpha3.PublishConfig{
						Destination: zarfv1alpha3.PublishDestination{
							Type: zarfv1alpha3.DestinationTypeOCI,
						},
					},
				},
			},
			artifactPath: "/workspace/test.tar.zst",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dest := &OCIDestination{}
			cmd, err := dest.GetPublishCommand(tt.pkg, tt.artifactPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetPublishCommand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && cmd == "" {
				t.Error("GetPublishCommand() returned empty command")
			}
		})
	}
}

func TestOCIDestinationGetJobConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		pkg     *zarfv1alpha3.ZarfPackageJob
		wantErr bool
	}{
		{
			name: "oci without credentials",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Publish: &zarfv1alpha3.PublishConfig{
						Destination: zarfv1alpha3.PublishDestination{
							Type: zarfv1alpha3.DestinationTypeOCI,
							OCI: &zarfv1alpha3.OCIDestination{
								Registry:   "ghcr.io",
								Repository: "test/packages",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "oci with credentials",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Publish: &zarfv1alpha3.PublishConfig{
						Destination: zarfv1alpha3.PublishDestination{
							Type: zarfv1alpha3.DestinationTypeOCI,
							OCI: &zarfv1alpha3.OCIDestination{
								Registry:   "ghcr.io",
								Repository: "test/packages",
								CredentialRef: &common.SecretReference{
									Name: "registry-creds",
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dest := &OCIDestination{}
			config, err := dest.GetJobConfiguration(tt.pkg)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetJobConfiguration() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && config == nil {
				t.Error("GetJobConfiguration() returned nil config")
			}
		})
	}
}

func TestLocalDestinationGetPublishCommand(t *testing.T) {
	tests := []struct {
		name         string
		pkg          *zarfv1alpha3.ZarfPackageJob
		artifactPath string
		wantErr      bool
	}{
		{
			name: "local copy",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pkg",
					Namespace: "default",
				},
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Publish: &zarfv1alpha3.PublishConfig{
						Destination: zarfv1alpha3.PublishDestination{
							Type: zarfv1alpha3.DestinationTypeLocal,
							Local: &zarfv1alpha3.LocalDestination{
								Path: "/output/packages",
							},
						},
					},
				},
			},
			artifactPath: "/workspace/test.tar.zst",
			wantErr:      true, // local destination requires devMode
		},
		{
			name: "missing local config",
			pkg: &zarfv1alpha3.ZarfPackageJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pkg",
					Namespace: "default",
				},
				Spec: zarfv1alpha3.ZarfPackageJobSpec{
					Publish: &zarfv1alpha3.PublishConfig{
						Destination: zarfv1alpha3.PublishDestination{
							Type: zarfv1alpha3.DestinationTypeLocal,
						},
					},
				},
			},
			artifactPath: "/workspace/test.tar.zst",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dest := &LocalDestination{}
			cmd, err := dest.GetPublishCommand(tt.pkg, tt.artifactPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetPublishCommand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && cmd == "" {
				t.Error("GetPublishCommand() returned empty command")
			}
		})
	}
}

func TestLocalDestinationGetJobConfiguration(t *testing.T) {
	dest := &LocalDestination{}
	pkg := &zarfv1alpha3.ZarfPackageJob{
		Spec: zarfv1alpha3.ZarfPackageJobSpec{
			Publish: &zarfv1alpha3.PublishConfig{
				Destination: zarfv1alpha3.PublishDestination{
					Type: zarfv1alpha3.DestinationTypeLocal,
					Local: &zarfv1alpha3.LocalDestination{
						Path: "/output",
					},
				},
			},
		},
	}

	config, err := dest.GetJobConfiguration(pkg)
	if err != nil {
		t.Errorf("GetJobConfiguration() error = %v", err)
		return
	}
	if config == nil {
		t.Error("GetJobConfiguration() returned nil config")
	}
}
