# Deployment ownership and bootstrap

SignalDock uses three management layers. Keeping their responsibilities separate avoids duplicate ownership of the same resource.

| Layer | Owns |
| --- | --- |
| Terraform | AWS VPC/subnets, EKS cluster and node group, EKS managed add-ons, IAM roles/policies, Pod Identity associations, AWS Load Balancer Controller permissions, and AWS Secrets Manager secret containers |
| Argo CD / GitOps | SignalDock Helm release, ExternalSecret/SecretStore resources, AWS `gp3` StorageClass, AWS Load Balancer Controller, and the AWS ImageUpdater custom resource |
| Manual bootstrap | Initial controller installation, secret values, and the Git credential used by Image Updater |

Secret values and Git credentials are intentionally not stored in Git or Terraform state.

## AWS bootstrap

The versions below are pinned so a new cluster can be reproduced instead of depending on a moving `stable` or `latest` reference.

The AWS Load Balancer Controller is managed by Argo CD and pinned to Helm chart `1.14.0` (controller `v2.14.1`).

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

### 4. Bootstrap GitOps applications

The platform Application adopts and manages the AWS in-cluster resources under `deploy/aws/`, including the `gp3` StorageClass:

```bash
kubectl apply -f deploy/argocd/aws-platform-application.yaml
kubectl apply -f deploy/argocd/aws-load-balancer-controller-application.yaml
kubectl apply -f deploy/argocd/external-secrets-aws-application.yaml
```

Wait for External Secrets to materialize the Kubernetes Secrets:

```bash
kubectl get externalsecret -n signaldock
kubectl get secret -n signaldock
```

Then deploy SignalDock:

```bash
kubectl apply -f deploy/argocd/signaldock-aws-application.yaml
```

### 5. Configure Image Updater Git write-back

Create the Git credential directly in the cluster. Never commit this Secret:

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

Apply the ImageUpdater resource:

```bash
kubectl apply -f deploy/argocd/signaldock-image-updater-aws.yaml
```

Only the AWS Image Updater writes image tags back to `deploy/helm/signaldock/values.yaml`. Local clusters consume those Git changes but do not perform write-back.

### 6. Verify

```bash
kubectl get application -n argocd
kubectl get deployment aws-load-balancer-controller -n kube-system
kubectl get imageupdater -n argocd
kubectl get pods -n signaldock
kubectl get pvc -n signaldock
kubectl get externalsecret -n signaldock
```

The expected final state is `Synced` / `Healthy` for the Argo CD Applications and `Running` for the SignalDock workloads.

## Local cluster

The local environment uses the same Helm chart and common `values.yaml`. No local values override is currently needed.

A fresh local cluster still needs the Argo CD and External Secrets Operator controllers installed first. The Image Updater controller is not required locally because AWS is the single Git write-back owner.

Local External Secrets reads from manually created source Secrets in the `shared-secrets` namespace through the Kubernetes provider. Create those source Secrets out of band, then apply:

```bash
kubectl apply -f deploy/argocd/external-secrets-application.yaml
kubectl apply -f deploy/argocd/signaldock-application.yaml
```

The local cluster intentionally has no ImageUpdater writer. This prevents local and AWS controllers from competing to commit changes to the same Helm values file.
