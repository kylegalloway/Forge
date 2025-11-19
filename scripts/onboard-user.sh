#!/bin/bash
#
# User Onboarding Script for ScriptRunner
#
# This script creates a new user namespace with:
# - Pod Security Standards
# - ResourceQuota
# - LimitRange
# - RBAC permissions
#
# Usage: ./scripts/onboard-user.sh <username> [options]
#
# Options:
#   --max-scriptrunners NUM    Maximum ScriptRunner resources (default: 50)
#   --max-jobs NUM             Maximum concurrent Jobs (default: 20)
#   --cpu-request NUM          CPU request quota (default: 5)
#   --cpu-limit NUM            CPU limit quota (default: 10)
#   --memory-request SIZE      Memory request quota (default: 10Gi)
#   --memory-limit SIZE        Memory limit quota (default: 20Gi)
#   --dry-run                  Show what would be created without creating it
#

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Default values
MAX_SCRIPTRUNNERS=50
MAX_JOBS=20
MAX_PODS=20
CPU_REQUEST="5"
CPU_LIMIT="10"
MEMORY_REQUEST="10Gi"
MEMORY_LIMIT="20Gi"
STORAGE_REQUEST="10Gi"
MAX_PVCS=5
POD_MAX_CPU="4"
POD_MAX_MEMORY="8Gi"
POD_MIN_CPU="50m"
POD_MIN_MEMORY="64Mi"
CONTAINER_MAX_CPU="2"
CONTAINER_MAX_MEMORY="4Gi"
CONTAINER_MIN_CPU="50m"
CONTAINER_MIN_MEMORY="64Mi"
CONTAINER_DEFAULT_LIMIT_CPU="1"
CONTAINER_DEFAULT_LIMIT_MEMORY="1Gi"
CONTAINER_DEFAULT_REQUEST_CPU="250m"
CONTAINER_DEFAULT_REQUEST_MEMORY="256Mi"
PVC_MAX_STORAGE="5Gi"
PVC_MIN_STORAGE="1Gi"
DRY_RUN=""

# Parse arguments
if [ -z "$1" ]; then
    echo -e "${RED}Error: Username required${NC}"
    echo "Usage: $0 <username> [options]"
    exit 1
fi

USERNAME="$1"
shift

while [[ $# -gt 0 ]]; do
    case $1 in
        --max-scriptrunners)
            MAX_SCRIPTRUNNERS="$2"
            shift 2
            ;;
        --max-jobs)
            MAX_JOBS="$2"
            shift 2
            ;;
        --cpu-request)
            CPU_REQUEST="$2"
            shift 2
            ;;
        --cpu-limit)
            CPU_LIMIT="$2"
            shift 2
            ;;
        --memory-request)
            MEMORY_REQUEST="$2"
            shift 2
            ;;
        --memory-limit)
            MEMORY_LIMIT="$2"
            shift 2
            ;;
        --dry-run)
            DRY_RUN="--dry-run=client"
            shift
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            exit 1
            ;;
    esac
done

echo -e "${GREEN}ScriptRunner User Onboarding${NC}"
echo "================================"
echo "Username: $USERNAME"
echo "Namespace: user-$USERNAME"
echo "Max ScriptRunners: $MAX_SCRIPTRUNNERS"
echo "Max Jobs: $MAX_JOBS"
echo "CPU Request: $CPU_REQUEST"
echo "CPU Limit: $CPU_LIMIT"
echo "Memory Request: $MEMORY_REQUEST"
echo "Memory Limit: $MEMORY_LIMIT"
echo ""

if [ -n "$DRY_RUN" ]; then
    echo -e "${YELLOW}DRY RUN MODE - No resources will be created${NC}"
    echo ""
fi

# Check if namespace already exists
if kubectl get namespace "user-$USERNAME" &>/dev/null; then
    echo -e "${RED}Error: Namespace user-$USERNAME already exists${NC}"
    exit 1
fi

# Create temporary file for manifest
MANIFEST=$(mktemp)
trap "rm -f $MANIFEST" EXIT

# Generate manifest from template
cat > "$MANIFEST" <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: user-$USERNAME
  labels:
    pod-security.kubernetes.io/enforce: restricted
    pod-security.kubernetes.io/audit: restricted
    pod-security.kubernetes.io/warn: restricted
    scriptrunner.io/tenant: "$USERNAME"
    scriptrunner.io/managed: "true"
    scriptrunner.io/webhook: enabled
---
apiVersion: v1
kind: ResourceQuota
metadata:
  name: scriptrunner-quota
  namespace: user-$USERNAME
  labels:
    scriptrunner.io/tenant: "$USERNAME"
spec:
  hard:
    count/scriptrunners.scriptrunner.io: "$MAX_SCRIPTRUNNERS"
    count/jobs.batch: "$MAX_JOBS"
    pods: "$MAX_PODS"
    requests.cpu: "$CPU_REQUEST"
    limits.cpu: "$CPU_LIMIT"
    requests.memory: "$MEMORY_REQUEST"
    limits.memory: "$MEMORY_LIMIT"
    requests.storage: "$STORAGE_REQUEST"
    persistentvolumeclaims: "$MAX_PVCS"
---
apiVersion: v1
kind: LimitRange
metadata:
  name: scriptrunner-limits
  namespace: user-$USERNAME
  labels:
    scriptrunner.io/tenant: "$USERNAME"
spec:
  limits:
  - type: Pod
    max:
      cpu: "$POD_MAX_CPU"
      memory: "$POD_MAX_MEMORY"
    min:
      cpu: "$POD_MIN_CPU"
      memory: "$POD_MIN_MEMORY"
  - type: Container
    max:
      cpu: "$CONTAINER_MAX_CPU"
      memory: "$CONTAINER_MAX_MEMORY"
    min:
      cpu: "$CONTAINER_MIN_CPU"
      memory: "$CONTAINER_MIN_MEMORY"
    default:
      cpu: "$CONTAINER_DEFAULT_LIMIT_CPU"
      memory: "$CONTAINER_DEFAULT_LIMIT_MEMORY"
    defaultRequest:
      cpu: "$CONTAINER_DEFAULT_REQUEST_CPU"
      memory: "$CONTAINER_DEFAULT_REQUEST_MEMORY"
  - type: PersistentVolumeClaim
    max:
      storage: "$PVC_MAX_STORAGE"
    min:
      storage: "$PVC_MIN_STORAGE"
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: scriptrunner-user
  namespace: user-$USERNAME
  labels:
    scriptrunner.io/tenant: "$USERNAME"
    scriptrunner.io/rbac: "user"
rules:
# Full access to ScriptRunner resources
- apiGroups: ["scriptrunner.io"]
  resources: ["scriptrunners"]
  verbs: ["create", "get", "list", "watch", "update", "patch", "delete"]
# Read-only access to ScriptRunner status
- apiGroups: ["scriptrunner.io"]
  resources: ["scriptrunners/status"]
  verbs: ["get", "list", "watch"]
# Read-only access to Jobs (created by controller)
- apiGroups: ["batch"]
  resources: ["jobs", "jobs/status"]
  verbs: ["get", "list", "watch"]
# Read-only access to Pods (for debugging)
- apiGroups: [""]
  resources: ["pods", "pods/log"]
  verbs: ["get", "list", "watch"]
# Read events (for debugging)
- apiGroups: [""]
  resources: ["events"]
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: $USERNAME-scriptrunner-user
  namespace: user-$USERNAME
  labels:
    scriptrunner.io/tenant: "$USERNAME"
    scriptrunner.io/rbac: "user"
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: scriptrunner-user
subjects:
- kind: User
  name: "$USERNAME"
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: deny-all-ingress
  namespace: user-$USERNAME
  labels:
    scriptrunner.io/tenant: "$USERNAME"
    scriptrunner.io/network-policy: "default-deny"
spec:
  podSelector: {}
  policyTypes:
  - Ingress
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-dns
  namespace: user-$USERNAME
  labels:
    scriptrunner.io/tenant: "$USERNAME"
    scriptrunner.io/network-policy: "dns"
spec:
  podSelector: {}
  policyTypes:
  - Egress
  egress:
  - to:
    - namespaceSelector:
        matchLabels:
          kubernetes.io/metadata.name: kube-system
    - podSelector:
        matchLabels:
          k8s-app: kube-dns
    ports:
    - protocol: UDP
      port: 53
    - protocol: TCP
      port: 53
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-kubernetes-api
  namespace: user-$USERNAME
  labels:
    scriptrunner.io/tenant: "$USERNAME"
    scriptrunner.io/network-policy: "api-access"
spec:
  podSelector: {}
  policyTypes:
  - Egress
  egress:
  - to:
    - namespaceSelector:
        matchLabels:
          kubernetes.io/metadata.name: default
    ports:
    - protocol: TCP
      port: 443
    - protocol: TCP
      port: 6443
EOF

# Apply manifest
echo -e "${GREEN}Creating namespace and resources...${NC}"
kubectl apply -f "$MANIFEST" $DRY_RUN

if [ -z "$DRY_RUN" ]; then
    echo ""
    echo -e "${GREEN}✓ User onboarding complete!${NC}"
    echo ""
    echo "Namespace: user-$USERNAME"
    echo "User: $USERNAME"
    echo ""
    echo "Next steps:"
    echo "1. Share kubeconfig with user (scoped to their namespace)"
    echo "2. Provide user documentation: docs/USER_GUIDE.md"
    echo "3. Show approved scripts list"
    echo ""
    echo "User can now create ScriptRunners in their namespace:"
    echo "  kubectl apply -f my-scriptrunner.yaml -n user-$USERNAME"
    echo ""
    echo "View quota usage:"
    echo "  kubectl describe resourcequota scriptrunner-quota -n user-$USERNAME"
else
    echo ""
    echo -e "${YELLOW}Dry run complete - no resources created${NC}"
fi
