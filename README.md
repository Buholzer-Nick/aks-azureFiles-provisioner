# azurefile-provisioner

Custom Azure File share provisioner for AKS, built with controller-runtime.

## Local development
- Install Go 1.25.
- Run locally against a cluster:
  - `kind create cluster` or `minikube start`
  - `export KUBECONFIG=~/.kube/config`
  - `go run ./cmd/manager`

## Build and test
- `make fmt` : format Go sources.
- `make test` : run unit tests.
- `make lint` : placeholder for lint tooling.

## Run in cluster
- Build and push your image, then deploy using your preferred tooling (kustomize or helm).
- Ensure RBAC grants only the permissions required for PVC/PV, Events, and related resources.

## Deploy with kustomize
1) Build and push your image, then update `deploy/kustomize/deployment.yaml` to point at it.
2) Set Azure config values in `deploy/kustomize/configmap.yaml` (resource group, storage account, server).
3) Apply the manifests:
   - `kubectl apply -k deploy/kustomize`
4) Verify the controller:
   - `kubectl -n azurefile-provisioner-system get pods`
   - `kubectl -n azurefile-provisioner-system logs deploy/azurefile-provisioner`

## Smoke test (AKS)
1) Apply the example StorageClass and PVC/Pod:
   - `kubectl apply -f deploy/examples/storageclass.yaml`
   - `kubectl apply -f deploy/examples/pvc.yaml`
   - `kubectl apply -f deploy/examples/pod.yaml`
2) Watch for binding:
   - `kubectl get pvc azurefile-demo -n default`
   - `kubectl get pv | grep azurefile-demo`
3) Confirm share creation (via Azure portal or CLI) and that the pod can mount `/data`.

## RBAC requirements
The controller needs cluster-scoped permissions to reconcile PVCs and bind PVs:
- PVCs: get/list/watch/update/patch (add finalizers and annotations).
- PVs: get/list/watch/create/update/delete (create and clean up PVs).
- StorageClasses: get/list/watch (to match the managed provisioner).
- Events: create/patch (emit lifecycle events).
See `config/rbac/role.yaml` for the minimal ClusterRole.
