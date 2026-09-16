# Deployment ownership and bootstrap

SignalDock uses three management layers. Keeping their responsibilities separate avoids duplicate ownership of the same resource.

| Layer | Owns |
| --- | --- |
| Terraform | AWS VPC/subnets, EKS cluster and node group, EKS managed add-ons, IAM roles/policies, Pod Identity associations, and AWS Secrets Manager secret containers |
| Argo CD / GitOps | SignalDock Helm release, ExternalSecret/SecretStore resources, AWS `gp3` StorageClass, and the AWS ImageUpdater custom resource |
| Manual bootstrap | Initial controller installation, secret values, Git credentials, and one root Argo CD Application per cluster |

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

Only the root Application is bootstrapped manually. After that, Argo CD manages the child Applications from Git with automated prune and self-heal enabled.

## AWS bootstrap

The versions below are pinned so a new cluster can be reproduced instead of depending on a moving `stable` or `latest` reference.

```bash
export ARGO_CD_VERSION=v3.5.3
export EXTERNAL_SECRETS_VERSION=2.10.0
export IMAGE_UPDATER_VERSION=v1.2.2
```

### 1. Provision AWS infrastructure

Authenticate the AWS CLI, then provision the infrastructure:

```bash
cd infra/terraform
terraform init
terraform plan
terraform apply
```

Configure kubectl from the Terraform-managed EKS cluster:

```bash
aws eks update-kubeconfig \
  --region eu-west-1 \
  --name "$(terraform output -raw eks_cluster_name)"
```

### 2. Install bootstrap controllers

Argo CD is the root GitOps controller, so its first installation is intentionally manual:

```bash
kubectl create namespace argocd
kubectl apply \
  --namespace argocd \
  --server-side \
  --force-conflicts \
  -f "https://raw.githubusercontent.com/argoproj/argo-cd/${ARGO_CD_VERSION}/manifests/install.yaml"
```

Install External Secrets Operator. Its ServiceAccount name must remain `external-secrets` because the Terraform Pod Identity association targets that namespace and ServiceAccount:

```bash
helm repo add external-secrets https://charts.external-secrets.io
helm repo update

helm upgrade --install external-secrets \
  external-secrets/external-secrets \
  --namespace external-secrets \
  --create-namespace \
  --version "${EXTERNAL_SECRETS_VERSION}"
```

Install Argo CD Image Updater in the same namespace as Argo CD:

```bash
kubectl apply \
  --namespace argocd \
  -f "https://raw.githubusercontent.com/argoproj-labs/argocd-image-updater/${IMAGE_UPDATER_VERSION}/config/install.yaml"
```

### 3. Populate external secret values

Terraform creates these AWS Secrets Manager containers but deliberately does not manage their values:

```text
signaldock/lab/signaldock
signaldock/lab/postgres
```

Populate them out of band before deploying the application. Do not commit the values or place them in Terraform variables/state.

### 4. Configure Image Updater Git write-back

Create the Git credential directly in the cluster before bootstrapping the root Application. Never commit this Secret:

```bash
read -s GITHUB_TOKEN

kubectl create secret generic image-updater-git-creds \
  --namespace argocd \
  --from-literal=username=sgrinan \
  --from-literal=password="$GITHUB_TOKEN" \
  --dry-run=client -o yaml \
  | kubectl apply -f -

unset GITHUB_TOKEN
```

Only the AWS Image Updater writes image tags back to `deploy/helm/signaldock/values.yaml`. Local clusters consume those Git changes but do not perform write-back.

### 5. Bootstrap GitOps

Apply the AWS root Application once:

```bash
kubectl apply -f deploy/argocd/bootstrap/aws-root-application.yaml
```

The root Application then manages:

- the AWS platform Application for `deploy/aws/` and the `gp3` StorageClass;
- the AWS External Secrets Application;
- the SignalDock Helm Application using `values-aws.yaml`;
- the AWS ImageUpdater custom resource.

Changes to those child manifests are subsequently reconciled directly from Git.

### 6. Verify

```bash
kubectl get application -n argocd
kubectl get imageupdater -n argocd
kubectl get pods -n signaldock
kubectl get pvc -n signaldock
kubectl get externalsecret -n signaldock
```

The expected AWS Applications are:

```text
signaldock-root-aws
signaldock-aws
signaldock-platform-aws
signaldock-secrets-aws
```

They should settle at `Synced` / `Healthy`, and the SignalDock workloads should be `Running`.

## Local cluster

The local environment uses the same Helm chart and common `values.yaml`. No local values override is currently needed.

A fresh local cluster needs Argo CD and External Secrets Operator installed first. The Image Updater controller is not required locally because AWS is the single Git write-back owner.

Local External Secrets reads from manually created source Secrets in the `shared-secrets` namespace through the Kubernetes provider. Create those source Secrets out of band, then bootstrap the local App of Apps once:

```bash
kubectl apply -f deploy/argocd/bootstrap/local-root-application.yaml
```

The root Application manages the local External Secrets and SignalDock Applications from Git. The local cluster intentionally has no ImageUpdater writer, preventing local and AWS controllers from competing to commit changes to the same Helm values file.
