# 🧠 Plan: Update Job Spec to Mount Projected SA Token for Kubeconfig

This plan explains **why** and **how** to modify a Kubernetes Job’s Pod template so that your containers can reliably get a service account token and CA cert via projected volumes — enabling your sidecar or task to generate a kubeconfig.

---

## 📌 Background

Kubernetes strongly encourages mounting **projected service account tokens** instead of long-lived secret tokens. With this approach you get short-lived, automatically refreshed tokens and access to namespace/CA data needed to build a kubeconfig for in-cluster API access. ([Kubernetes][1])

---

## 📚 Official References

To support these changes, reference the following Kubernetes docs:

* **Projected Volumes concept** — Shows how to define a `projected` volume with multiple sources, including `serviceAccountToken`. ([Kubernetes][1])
  [https://kubernetes.io/docs/concepts/storage/projected-volumes](https://kubernetes.io/docs/concepts/storage/projected-volumes)

* **Configure Service Accounts for Pods** — Explains how service account tokens are projected into Pods (via TokenRequest API) and how to mount them. ([Kubernetes][2])
  [https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account)

* **Jobs overview** — Clarifies that the Job’s `.spec.template` is a normal Pod spec which must include volumes/volumeMounts. ([Kubernetes][3])
  [https://kubernetes.io/docs/concepts/workloads/controllers/job](https://kubernetes.io/docs/concepts/workloads/controllers/job)

---

## 🧾 What to Add to the Job Spec

Update the `.spec.template.spec` of your Job to include:

### 1️⃣ Service Account

```yaml
serviceAccountName: my-sa
```

Ensure this SA has RBAC permissions for the cluster actions you need.

---

### 2️⃣ Projected Volume (for token, CA, namespace)

Add a projected volume named (e.g.) `kube-api-access`:

```yaml
volumes:
- name: kube-api-access
  projected:
    sources:
    - serviceAccountToken:
        path: token
        expirationSeconds: 3600
        audience: https://kubernetes.default.svc
    - configMap:
        name: kube-root-ca.crt
        items:
        - key: ca.crt
          path: ca.crt
    - downwardAPI:
        items:
        - path: namespace
          fieldRef:
            fieldPath: metadata.namespace
```

This gives tokens and certs inside the Pod without needing legacy Secret mounts. ([Kubernetes][1])

---

### 3️⃣ Mount It In Your Container

Mount it under the traditional serviceaccount path:

```yaml
containers:
- name: kubectl
  volumeMounts:
  - name: kube-api-access
    mountPath: /var/run/secrets/kubernetes.io/serviceaccount
    readOnly: true
```

Mounting here lets tools like `kubectl` (or your kubeconfig builder) find the token, CA cert, and namespace in the expected location.

---

## 🧪 Test After Applying

After updating the Job manifest:

1. Delete any old pods so the Job will recreate them:

   ```bash
   kubectl delete pod -l job-name=<your-job-name>
   ```

2. Inside the new pod:

   ```bash
   ls /var/run/secrets/kubernetes.io/serviceaccount
   cat /var/run/secrets/kubernetes.io/serviceaccount/namespace
   ```

   You should see the `token`, `ca.crt`, and `namespace` files.

---

## 📌 Why This Works

* Kubernetes projects **service account tokens** into pods via the TokenRequest API, and uses a `projected` volume to combine token, CA, and downward API data in one mount. ([Kubernetes][2])
* This is the recommended approach instead of mounting static secret tokens. ([Kubernetes][2])

---

[1]: https://kubernetes.io/docs/concepts/storage/projected-volumes?utm_source=chatgpt.com "Projected Volumes | Kubernetes"
[2]: https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/?utm_source=chatgpt.com "Configure Service Accounts for Pods | Kubernetes"
[3]: https://kubernetes.io/docs/concepts/workloads/controllers/job/?utm_source=chatgpt.com "Jobs | Kubernetes"
