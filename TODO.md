# TODO

- Wire real Azure share lifecycle in reconcile: ensure quota updates, handle share expansion, and classify retryable vs terminal Azure errors.
- Add StorageClass/PVC validation: required params, supported access modes, and explicit user-facing events for invalid inputs.
- Decide PV mismatch remediation policy (recreate vs halt) and emit a dedicated event.
- Expand metrics: add `result,phase` labels and optional delete/cleanup counters.
- Add Azure Workload Identity setup notes and ServiceAccount annotations for federated credentials.
- Enforce required config (resource group/storage account) at startup with a clear error path.
- Add controller tests for ShareClient error classes and StorageClass parameter parsing.
- Add kustomize overlays (config/manager) or optional Helm chart if needed by deployment workflows.
