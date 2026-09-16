# Deployment ownership and bootstrap

SignalDock uses three management layers. Keeping their responsibilities separate avoids duplicate ownership of the same resource.

| Layer | Owns |
| --- | --- |
| Terraform | AWS VPC/subnets, EKS cluster and node group, EKS managed add-ons, IAM roles/policies, Pod Identity associations, and AWS Secrets Manager secret containers |
| Argo CD / GitOps | SignalDock Helm release, ExternalSecret/SecretStore resources, AWS `gp3` StorageClass, and the AWS ImageUpdater custom resource |
| Bootstrap / operator input | Initial controller installation, secret values, Git credentials, and one root Argo CD Application per cluster |

Secret values and Git credentials are intentionally not stored in Git or Terraform state.

## Argo CD layout

The Argo CD manifests use an App of Apps layout so changes to child Applications are reconciled from Git instead of requiring repeated `kubectl apply` commands.

```text
deploy/argocd/
├── bootstrap/
│   ├── aws-root-application.yaml
│   └── local-root-application.yaml
├── aws/
│   ├── aws-platform-application.yaml
│   ├── external-secrets-aws-application.yaml
│   ├── signaldock-aws-application.yaml
│   └── signaldock-image-updater-aws.yaml
└── local/
    ├── external-secrets-application.yaml
    └── signaldock-application.yaml
```

Only the root Application is bootstrapped explicitly. After that, Argo CD manages the child Applications from Git with automated prune and self-heal enabled.

## AWS deployment

The repository Makefile provides the supported workflow for provisioning, bootstrapping, inspecting, and destroying the AWS environment.

The main operator-facing targets are:

```text
make aws-up
make aws-bootstrap
make aws-status
make aws-destroy
```

Supporting targets are available for lower-level operations:

```text
make aws-init
make aws-plan
make aws-kubeconfig
make aws-secrets-status
```

Argo CD, External Secrets Operator, and Argo CD Image Updater versions are pinned in the Makefile so a fresh cluster does not depend on moving `stable` or `latest` references.

### 1. Provision AWS infrastructure

Authenticate the AWS CLI with the configured profile, then run:

```bash
make aws-up
```

By default, the Makefile uses:

```text
AWS_PROFILE=signaldock
AWS_REGION=eu-west-1
EKS_CLUSTER=signaldock-lab
EKS_CONTEXT=signaldock-aws
```

These values can be overridden without editing the repository:

```bash
make aws-up AWS_PROFILE=my-profile AWS_REGION=eu-west-1
```

`aws-up` performs the infrastructure phase only:

```text
AWS credential check
        ↓
terraform init
        ↓
terraform apply
        ↓
EKS kubeconfig configuration
```

Terraform creates the AWS infrastructure and Secrets Manager secret containers. It deliberately does not manage the secret values.

### 2. Populate external secret values

Terraform creates these AWS Secrets Manager containers:

```text
signaldock/lab/signaldock
signaldock/lab/postgres
```

Terraform deliberately does not create their current secret values. Populate both secrets directly in AWS Secrets Manager before bootstrapping the workloads.

The AWS ExternalSecret resources use `dataFrom.extract`, so each Secrets Manager value must contain the expected key/value fields.

`signaldock/lab/signaldock`:

```text
SIGNALDOCK_ADMIN_USERNAME=<admin-username>
SIGNALDOCK_ADMIN_PASSWORD=<admin-password>
```

Equivalent JSON value:

```json
{
  "SIGNALDOCK_ADMIN_USERNAME": "<admin-username>",
  "SIGNALDOCK_ADMIN_PASSWORD": "<admin-password>"
}
```

`signaldock/lab/postgres`:

```text
POSTGRES_DB=<database-name>
POSTGRES_USER=<database-username>
POSTGRES_PASSWORD=<database-password>
```

Equivalent JSON value:

```json
{
  "POSTGRES_DB": "<database-name>",
  "POSTGRES_USER": "<database-username>",
  "POSTGRES_PASSWORD": "<database-password>"
}
```

Do not commit these values or place them in `.env`, Terraform variables, or Terraform state. The root `.env` file is only for the Docker Compose environment and is not used by AWS or Kubernetes.

Their availability can be checked without printing their contents:

```bash
make aws-secrets-status
```

### 3. Bootstrap the cluster

After the AWS secret values have been populated, run:

```bash
make aws-bootstrap
```

The bootstrap target:

```text
configures the EKS kubectl context
        ↓
installs Argo CD
        ↓
installs External Secrets Operator
        ↓
installs Argo CD Image Updater
        ↓
checks the required Secrets Manager values
        ↓
creates the Image Updater Git credential if missing
        ↓
applies the AWS root Argo CD Application
```

External Secrets Operator uses the `external-secrets` ServiceAccount because the Terraform-managed Pod Identity association targets that namespace and ServiceAccount.

If the Image Updater Git credential does not already exist, the bootstrap target requests the GitHub PAT interactively with hidden input and creates:

```text
argocd/image-updater-git-creds
```

The token is not stored in the Makefile or repository.

Only the AWS Image Updater writes image tags back to:

```text
deploy/helm/signaldock/values.yaml
```

Local clusters consume those Git changes but do not perform write-back.

### 4. GitOps reconciliation

The bootstrap target applies:

```text
deploy/argocd/bootstrap/aws-root-application.yaml
```

The root Application then manages:

- the AWS platform Application for `deploy/aws/` and the `gp3` StorageClass;
- the AWS External Secrets Application;
- the SignalDock Helm Application using `values-aws.yaml`;
- the AWS ImageUpdater custom resource.

The expected AWS Applications are:

```text
signaldock-root-aws
signaldock-aws
signaldock-platform-aws
signaldock-secrets-aws
```

Changes to those child manifests are subsequently reconciled directly from Git.

### 5. Verify

Run:

```bash
make aws-status
```

The status target checks:

```text
EKS nodes
Argo CD Applications
Argo CD Image Updater
SignalDock Pods
PersistentVolumeClaims
PersistentVolumes
ExternalSecrets
```

The Applications should settle at `Synced` / `Healthy`, and the SignalDock workloads should be `Running`.

### 6. Destroy the AWS lab

Destroy the complete lab with:

```bash
make aws-destroy
```

The command requires explicit confirmation:

```text
Type DESTROY to continue:
```

The teardown order is deliberate:

```text
stop the App of Apps reconciliation
        ↓
delete the child GitOps Applications
        ↓
delete the SignalDock namespace and PVCs
        ↓
wait for the associated PersistentVolumes to disappear
        ↓
verify the dynamically provisioned EBS volumes are gone in AWS
        ↓
terraform destroy
```

Before deleting Kubernetes resources, the target records the EBS-backed PersistentVolumes associated with SignalDock PVCs. If those PVs or EBS volumes are still present after the cleanup timeout, the target aborts **before** `terraform destroy` instead of silently leaving dynamic storage behind.

EBS volumes are not force-deleted with the AWS CLI. An unexpected leftover volume is treated as an error that should be inspected rather than hidden by an aggressive cleanup command.

## Local Kubernetes

### Direct Helm deployment

The same chart can be managed directly with the Makefile. When Helm manages the chart directly, the required `signaldock` and `postgres` Kubernetes Secrets must already exist in the `signaldock` namespace.

For a local smoke test, create the namespace and Secrets with your own values:

```bash
kubectl create namespace signaldock \
  --dry-run=client \
  -o yaml \
  | kubectl apply -f -

kubectl create secret generic postgres \
  --namespace signaldock \
  --from-literal=POSTGRES_DB=<database-name> \
  --from-literal=POSTGRES_USER=<database-username> \
  --from-literal=POSTGRES_PASSWORD=<database-password>

kubectl create secret generic signaldock \
  --namespace signaldock \
  --from-literal=SIGNALDOCK_ADMIN_USERNAME=<admin-username> \
  --from-literal=SIGNALDOCK_ADMIN_PASSWORD=<admin-password>
```

Then install and inspect the chart:

```bash
make helm-up
make helm-status
```

Remove the Helm release with:

```bash
make helm-down
```

These manually created target Secrets are only for direct Helm management. The GitOps deployment obtains the same target Secrets through External Secrets instead.

### Local GitOps deployment

The local GitOps environment uses the same Helm chart and common `values.yaml`. No local values override is currently needed.

A fresh local GitOps cluster needs Argo CD and External Secrets Operator installed first. Argo CD Image Updater is not required locally because AWS is the single Git write-back owner.

Local External Secrets reads from manually created source Secrets in the `shared-secrets` namespace through the Kubernetes provider. These source Secrets use the same key names as Docker Compose and AWS Secrets Manager, but they are intentionally separate from the target `signaldock` and `postgres` Secrets created by ESO.

For a fresh local GitOps environment, create the source namespace and Secrets with your own values:

```bash
kubectl create namespace shared-secrets \
  --dry-run=client \
  -o yaml \
  | kubectl apply -f -

kubectl create secret generic postgres-source \
  --namespace shared-secrets \
  --from-literal=POSTGRES_DB=<database-name> \
  --from-literal=POSTGRES_USER=<database-username> \
  --from-literal=POSTGRES_PASSWORD=<database-password>

kubectl create secret generic signaldock-source \
  --namespace shared-secrets \
  --from-literal=SIGNALDOCK_ADMIN_USERNAME=<admin-username> \
  --from-literal=SIGNALDOCK_ADMIN_PASSWORD=<admin-password>
```

Then bootstrap the local App of Apps once:

```bash
kubectl apply -f deploy/argocd/bootstrap/local-root-application.yaml
```

The root Application manages the local External Secrets and SignalDock Applications from Git. The local cluster intentionally has no ImageUpdater writer, preventing local and AWS controllers from competing to commit changes to the same Helm values file.
