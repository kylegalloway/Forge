package credentials

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetSecret_Git_SSHKey(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "git-creds",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"ssh-key": []byte("fake-ssh-key"),
		},
	}

	client := fake.NewClientset(secret)
	manager := NewManager(client)

	cred, err := manager.GetSecret(context.Background(), "default", "git-creds", TypeGit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cred.Type != TypeGit {
		t.Errorf("expected type %s, got %s", TypeGit, cred.Type)
	}

	if string(cred.Data["ssh-key"]) != "fake-ssh-key" {
		t.Errorf("unexpected ssh-key value")
	}
}

func TestGetSecret_Git_Token(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "git-creds",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"token": []byte("ghp_fake_token"),
		},
	}

	client := fake.NewClientset(secret)
	manager := NewManager(client)

	cred, err := manager.GetSecret(context.Background(), "default", "git-creds", TypeGit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(cred.Data["token"]) != "ghp_fake_token" {
		t.Errorf("unexpected token value")
	}
}

func TestGetSecret_Git_Invalid(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "git-creds",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"password": []byte("wrong-key"),
		},
	}

	client := fake.NewClientset(secret)
	manager := NewManager(client)

	_, err := manager.GetSecret(context.Background(), "default", "git-creds", TypeGit)
	if err == nil {
		t.Fatal("expected error for invalid git secret, got nil")
	}

	if err.Error() != "invalid secret default/git-creds: git secret must contain 'ssh-key' or 'token'" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGetSecret_S3_Valid(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "s3-creds",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"access-key-id":     []byte("AKIAIOSFODNN7EXAMPLE"),                     // pragma: allowlist secret
			"secret-access-key": []byte("wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"), // pragma: allowlist secret
		},
	}

	client := fake.NewClientset(secret)
	manager := NewManager(client)

	cred, err := manager.GetSecret(context.Background(), "default", "s3-creds", TypeS3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cred.Type != TypeS3 {
		t.Errorf("expected type %s, got %s", TypeS3, cred.Type)
	}
}

func TestGetSecret_S3_MissingAccessKey(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "s3-creds",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"secret-access-key": []byte("wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"), // pragma: allowlist secret
		},
	}

	client := fake.NewClientset(secret)
	manager := NewManager(client)

	_, err := manager.GetSecret(context.Background(), "default", "s3-creds", TypeS3)
	if err == nil {
		t.Fatal("expected error for missing access-key-id, got nil")
	}
}

func TestGetSecret_S3_MissingSecretKey(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "s3-creds",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"access-key-id": []byte("AKIAIOSFODNN7EXAMPLE"), // pragma: allowlist secret
		},
	}

	client := fake.NewClientset(secret)
	manager := NewManager(client)

	_, err := manager.GetSecret(context.Background(), "default", "s3-creds", TypeS3)
	if err == nil {
		t.Fatal("expected error for missing secret-access-key, got nil")
	}
}

func TestGetSecret_OCI_Valid(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "oci-creds",
			Namespace: "default",
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			".dockerconfigjson": []byte(`{"auths":{"ghcr.io":{"auth":"dXNlcjpwYXNz"}}}`),
		},
	}

	client := fake.NewClientset(secret)
	manager := NewManager(client)

	cred, err := manager.GetSecret(context.Background(), "default", "oci-creds", TypeOCI)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cred.Type != TypeOCI {
		t.Errorf("expected type %s, got %s", TypeOCI, cred.Type)
	}
}

func TestGetSecret_OCI_Missing(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "oci-creds",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"username": []byte("user"),
			"password": []byte("pass"),
		},
	}

	client := fake.NewClientset(secret)
	manager := NewManager(client)

	_, err := manager.GetSecret(context.Background(), "default", "oci-creds", TypeOCI)
	if err == nil {
		t.Fatal("expected error for missing .dockerconfigjson, got nil")
	}
}

func TestGetSecret_NotFound(t *testing.T) {
	client := fake.NewClientset()
	manager := NewManager(client)

	_, err := manager.GetSecret(context.Background(), "default", "nonexistent", TypeGit)
	if err == nil {
		t.Fatal("expected error for nonexistent secret, got nil")
	}
}
