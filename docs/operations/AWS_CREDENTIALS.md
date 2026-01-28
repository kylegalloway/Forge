# AWS Credentials Configuration

Forge supports two methods for AWS authentication when accessing S3 sources or destinations:

1. **Static credentials** - Access key and secret key stored in a Kubernetes Secret
2. **IRSA (IAM Roles for Service Accounts)** - Pod assumes an IAM role via service account

## Method 1: Static Credentials (EnvVar)

Create a secret with your AWS credentials:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: aws-credentials
  namespace: forge-jobs
type: Opaque
stringData:
  # pragma: allowlist nextline secret
  access-key-id: "AKIAIOSFODNN7EXAMPLE"
  # pragma: allowlist nextline secret
  secret-access-key: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
```

Reference in your job:

```yaml
apiVersion: forge.dev/v1alpha3
kind: ZarfPackageJob
metadata:
  name: build-to-s3
  namespace: forge-jobs
spec:
  serviceAccountName: forge-builder
  action: BuildPublish
  source:
    type: Git
    git:
      url: https://github.com/example/zarf-package.git
  publish:
    destination:
      type: S3
      s3:
        bucket: my-artifacts-bucket
        keyPrefix: packages/
        region: us-east-1
        credentialRef:
          type: EnvVar  # Load as environment variables
          name: aws-credentials
```

## Method 2: IRSA (IAM Roles for Service Accounts)

IRSA allows pods to assume IAM roles without static credentials. This is the recommended approach for EKS clusters.

### Prerequisites

- EKS cluster with IRSA enabled (OIDC provider configured)
- IAM role with appropriate S3 permissions

### Step 1: Create IAM Policy

Create an IAM policy with the required S3 permissions:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "s3:PutObject",
        "s3:GetObject",
        "s3:ListBucket"
      ],
      "Resource": [
        "arn:aws:s3:::my-artifacts-bucket",
        "arn:aws:s3:::my-artifacts-bucket/*"
      ]
    }
  ]
}
```

```bash
aws iam create-policy \
  --policy-name ForgeS3Access \
  --policy-document file://policy.json
```

### Step 2: Create IAM Role with Trust Policy

Create a trust policy that allows your EKS cluster's OIDC provider to assume the role:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:aws:iam::ACCOUNT_ID:oidc-provider/oidc.eks.REGION.amazonaws.com/id/OIDC_ID"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "oidc.eks.REGION.amazonaws.com/id/OIDC_ID:sub": "system:serviceaccount:forge-jobs:forge-s3-access",
          "oidc.eks.REGION.amazonaws.com/id/OIDC_ID:aud": "sts.amazonaws.com"
        }
      }
    }
  ]
}
```

Replace:
- `ACCOUNT_ID`: Your AWS account ID
- `REGION`: Your EKS cluster region
- `OIDC_ID`: Your EKS cluster's OIDC provider ID
- `forge-jobs`: Namespace where jobs run
- `forge-s3-access`: Service account name

```bash
# Get your OIDC provider ID
aws eks describe-cluster --name my-cluster --query "cluster.identity.oidc.issuer" --output text

# Create the role
aws iam create-role \
  --role-name ForgeS3Role \
  --assume-role-policy-document file://trust-policy.json

# Attach the policy
aws iam attach-role-policy \
  --role-name ForgeS3Role \
  --policy-arn arn:aws:iam::ACCOUNT_ID:policy/ForgeS3Access
```

### Step 3: Create Kubernetes Service Account

Create a service account annotated with the IAM role ARN:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: forge-s3-access
  namespace: forge-jobs
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::ACCOUNT_ID:role/ForgeS3Role
```

### Step 4: Use in ZarfPackageJob/UDSBundleJob

Reference the service account and use `Node` credential type:

```yaml
apiVersion: forge.dev/v1alpha3
kind: ZarfPackageJob
metadata:
  name: build-to-s3-irsa
  namespace: forge-jobs
spec:
  # Use the IRSA-configured service account
  serviceAccountName: forge-s3-access
  action: BuildPublish
  source:
    type: Git
    git:
      url: https://github.com/example/zarf-package.git
  publish:
    destination:
      type: S3
      s3:
        bucket: my-artifacts-bucket
        keyPrefix: packages/
        region: us-east-1
        credentialRef:
          type: Node  # Use IRSA - no secret needed
```

The same applies to S3 sources:

```yaml
apiVersion: forge.dev/v1alpha3
kind: ZarfPackageJob
metadata:
  name: deploy-from-s3-irsa
  namespace: forge-jobs
spec:
  serviceAccountName: forge-s3-access
  action: Deploy
  source:
    type: S3
    s3:
      bucket: my-artifacts-bucket
      key: packages/my-package.tar.zst
      region: us-east-1
      credentialRef:
        type: Node  # Use IRSA
  deploy:
    target: InCluster
```

## UDS Bundle Example with IRSA

```yaml
apiVersion: forge.dev/v1alpha3
kind: UDSBundleJob
metadata:
  name: create-bundle-to-s3
  namespace: forge-jobs
spec:
  serviceAccountName: forge-s3-access
  action: CreatePublish
  source:
    type: Git
    git:
      url: https://github.com/example/uds-bundle.git
  publish:
    destination:
      type: S3
      s3:
        bucket: my-bundles-bucket
        keyPrefix: bundles/
        region: us-west-2
        credentialRef:
          type: Node
```

## Credential Type Summary

| Type | Use Case | Secret Required | How It Works |
|------|----------|-----------------|--------------|
| `EnvVar` | Static credentials | Yes | Injects `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` from secret |
| `Node` | IRSA / Instance Profile | No | AWS SDK uses pod's service account token or EC2 metadata |

## Troubleshooting

### IRSA Not Working

1. **Verify service account annotation**:
   ```bash
   kubectl get sa forge-s3-access -n forge-jobs -o yaml
   ```
   Check for `eks.amazonaws.com/role-arn` annotation.

2. **Check pod has IRSA environment variables**:
   ```bash
   kubectl exec -it <pod-name> -n forge-jobs -- env | grep AWS
   ```
   Should show `AWS_ROLE_ARN` and `AWS_WEB_IDENTITY_TOKEN_FILE`.

3. **Verify IAM role trust policy**:
   - Ensure the OIDC provider ID matches your cluster
   - Ensure the service account namespace and name match

4. **Test credentials manually**:
   ```bash
   kubectl run aws-test --rm -it --image=amazon/aws-cli:latest \
     --overrides='{"spec":{"serviceAccountName":"forge-s3-access"}}' \
     -n forge-jobs -- sts get-caller-identity
   ```

### Static Credentials Not Working

1. **Verify secret exists**:
   ```bash
   kubectl get secret aws-credentials -n forge-jobs
   ```

2. **Check secret keys**:
   ```bash
   kubectl get secret aws-credentials -n forge-jobs -o jsonpath='{.data}' | jq
   ```
   Must have `access-key-id` and `secret-access-key` keys.

3. **Verify IAM user permissions**:
   Ensure the IAM user has the required S3 permissions.
