SHELL := /bin/bash

BINARY := bin/signaldock
PACKAGE := ./cmd/signaldock
IMAGE := signaldock

VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD)

LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT)

TEST_COMPOSE := docker compose -f compose.test.yaml
TEST_DATABASE_URL := postgres://signaldock:test@localhost:5433/signaldock_test?sslmode=disable

HELM_RELEASE := signaldock
HELM_NAMESPACE := signaldock
HELM_CHART := deploy/helm/signaldock

AWS_PROFILE ?= signaldock
AWS_REGION ?= eu-west-1
EKS_CLUSTER ?= signaldock-lab
EKS_CONTEXT ?= signaldock-aws

TF_DIR := infra/terraform

ARGO_CD_VERSION ?= v3.5.3
EXTERNAL_SECRETS_VERSION ?= 2.10.0
IMAGE_UPDATER_VERSION ?= v1.2.2

AWS_ROOT_APPLICATION := deploy/argocd/bootstrap/aws-root-application.yaml

ARGO_ROOT_APPLICATION := signaldock-root-aws
ARGO_CHILD_APPLICATIONS := signaldock-aws signaldock-platform-aws signaldock-secrets-aws

SIGNALDOCK_AWS_SECRET := signaldock/lab/signaldock
POSTGRES_AWS_SECRET := signaldock/lab/postgres

GITHUB_USERNAME ?= sgrinan
IMAGE_UPDATER_GIT_SECRET := image-updater-git-creds

KUBECTL_AWS := kubectl --context "$(EKS_CONTEXT)"


.PHONY: \
	build run test check ci fmt fmt-check vet clean \
	docker-build up down status \
	helm-up helm-down helm-status helm-lint \
	aws-init aws-plan aws-kubeconfig aws-up aws-bootstrap \
	aws-secrets-status aws-status aws-destroy


# -----------------------------------------------------------------------------
# Go
# -----------------------------------------------------------------------------

build:
	mkdir -p bin
	go build -trimpath -ldflags "$(LDFLAGS)" -o "$(BINARY)" "$(PACKAGE)"


run:
	go run "$(PACKAGE)"


fmt:
	gofmt -w .


fmt-check:
	test -z "$$(gofmt -l .)"


vet:
	go vet ./...


test:
	@set -e; \
	trap '$(TEST_COMPOSE) down >/dev/null 2>&1' EXIT; \
	$(TEST_COMPOSE) up -d --wait; \
	SIGNALDOCK_TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test -race -p 1 ./...


check: fmt vet test


ci: fmt-check vet test build


clean:
	rm -rf bin


# -----------------------------------------------------------------------------
# Docker Compose
# -----------------------------------------------------------------------------

docker-build:
	docker build \
		--build-arg VERSION="$(VERSION)" \
		--build-arg COMMIT="$(COMMIT)" \
		-t "$(IMAGE):$(VERSION)" .


up:
	docker compose up -d --build


down:
	docker compose down


status:
	docker compose ps


# -----------------------------------------------------------------------------
# Local Kubernetes / Helm
# -----------------------------------------------------------------------------

helm-up:
	helm upgrade --install "$(HELM_RELEASE)" "$(HELM_CHART)" \
		--namespace "$(HELM_NAMESPACE)" \
		--create-namespace


helm-down:
	helm uninstall "$(HELM_RELEASE)" \
		--namespace "$(HELM_NAMESPACE)"


helm-status:
	helm status "$(HELM_RELEASE)" \
		--namespace "$(HELM_NAMESPACE)"


helm-lint:
	helm lint "$(HELM_CHART)"
	helm template "$(HELM_RELEASE)" "$(HELM_CHART)" >/dev/null


# -----------------------------------------------------------------------------
# AWS / Terraform / EKS
# -----------------------------------------------------------------------------

aws-init:
	@echo "Checking AWS credentials..."
	@AWS_PROFILE="$(AWS_PROFILE)" aws sts get-caller-identity >/dev/null
	@echo "Initializing Terraform..."
	@AWS_PROFILE="$(AWS_PROFILE)" terraform \
		-chdir="$(TF_DIR)" \
		init


aws-plan: aws-init
	@AWS_PROFILE="$(AWS_PROFILE)" terraform \
		-chdir="$(TF_DIR)" \
		plan


aws-kubeconfig:
	@echo "Configuring kubectl for $(EKS_CLUSTER)..."
	@aws eks update-kubeconfig \
		--profile "$(AWS_PROFILE)" \
		--region "$(AWS_REGION)" \
		--name "$(EKS_CLUSTER)" \
		--alias "$(EKS_CONTEXT)" >/dev/null
	@kubectl config use-context "$(EKS_CONTEXT)" >/dev/null
	@echo "Using Kubernetes context: $(EKS_CONTEXT)"


aws-up: aws-init
	@echo "Provisioning SignalDock AWS infrastructure..."
	@AWS_PROFILE="$(AWS_PROFILE)" terraform \
		-chdir="$(TF_DIR)" \
		apply
	@$(MAKE) aws-kubeconfig
	@echo
	@echo "AWS infrastructure is ready."
	@echo "Populate Secrets Manager values if required, then run:"
	@echo "  make aws-bootstrap"


# -----------------------------------------------------------------------------
# AWS bootstrap
# -----------------------------------------------------------------------------

aws-bootstrap: aws-kubeconfig
	@set -euo pipefail; \
	echo "Installing Argo CD $(ARGO_CD_VERSION)..."; \
	kubectl create namespace argocd \
		--dry-run=client \
		-o yaml \
		| kubectl apply -f - >/dev/null; \
	kubectl apply \
		--namespace argocd \
		--server-side \
		--force-conflicts \
		-f "https://raw.githubusercontent.com/argoproj/argo-cd/$(ARGO_CD_VERSION)/manifests/install.yaml"; \
	echo "Waiting for Argo CD CRDs..."; \
	kubectl wait \
		--for=condition=Established \
		crd/applications.argoproj.io \
		--timeout=180s; \
	echo "Installing External Secrets $(EXTERNAL_SECRETS_VERSION)..."; \
	helm repo add external-secrets \
		https://charts.external-secrets.io \
		--force-update >/dev/null; \
	helm repo update >/dev/null; \
	helm upgrade --install external-secrets \
		external-secrets/external-secrets \
		--namespace external-secrets \
		--create-namespace \
		--version "$(EXTERNAL_SECRETS_VERSION)"; \
	echo "Installing Argo CD Image Updater $(IMAGE_UPDATER_VERSION)..."; \
	kubectl apply \
		--namespace argocd \
		-f "https://raw.githubusercontent.com/argoproj-labs/argocd-image-updater/$(IMAGE_UPDATER_VERSION)/config/install.yaml"; \
	echo "Waiting for bootstrap controllers..."; \
	kubectl rollout status \
		deployment/argocd-server \
		-n argocd \
		--timeout=300s; \
	kubectl rollout status \
		deployment/external-secrets \
		-n external-secrets \
		--timeout=300s; \
	kubectl rollout status \
		deployment/argocd-image-updater \
		-n argocd \
		--timeout=300s; \
	echo "Checking AWS secret values..."; \
	missing=0; \
	for secret in \
		"$(SIGNALDOCK_AWS_SECRET)" \
		"$(POSTGRES_AWS_SECRET)"; do \
		if aws secretsmanager get-secret-value \
			--profile "$(AWS_PROFILE)" \
			--region "$(AWS_REGION)" \
			--secret-id "$$secret" >/dev/null 2>&1; then \
			echo "  OK: $$secret"; \
		else \
			echo "  MISSING VALUE: $$secret"; \
			missing=1; \
		fi; \
	done; \
	if [ "$$missing" -ne 0 ]; then \
		echo; \
		echo "Populate the missing Secrets Manager values and run:"; \
		echo "  make aws-bootstrap"; \
		exit 1; \
	fi; \
	if ! kubectl get secret "$(IMAGE_UPDATER_GIT_SECRET)" \
		-n argocd >/dev/null 2>&1; then \
		echo; \
		printf "GitHub PAT for Image Updater (input hidden): "; \
		read -r -s github_token; \
		echo; \
		kubectl create secret generic "$(IMAGE_UPDATER_GIT_SECRET)" \
			--namespace argocd \
			--from-literal=username="$(GITHUB_USERNAME)" \
			--from-literal=password="$$github_token"; \
		unset github_token; \
	else \
		echo "GitHub Image Updater credential already exists."; \
	fi; \
	echo "Bootstrapping SignalDock App of Apps..."; \
	kubectl apply -f "$(AWS_ROOT_APPLICATION)"; \
	echo; \
	echo "AWS bootstrap completed."; \
	echo "Run 'make aws-status' to check reconciliation."


aws-secrets-status:
	@set -e; \
	echo "AWS Secrets Manager:"; \
	for secret in \
		"$(SIGNALDOCK_AWS_SECRET)" \
		"$(POSTGRES_AWS_SECRET)"; do \
		if aws secretsmanager get-secret-value \
			--profile "$(AWS_PROFILE)" \
			--region "$(AWS_REGION)" \
			--secret-id "$$secret" >/dev/null 2>&1; then \
			echo "  OK      $$secret"; \
		else \
			echo "  MISSING $$secret"; \
		fi; \
	done


aws-status:
	@set -e; \
	echo "Checking AWS credentials..."; \
	aws sts get-caller-identity \
		--profile "$(AWS_PROFILE)" >/dev/null; \
	if ! aws eks describe-cluster \
		--profile "$(AWS_PROFILE)" \
		--region "$(AWS_REGION)" \
		--name "$(EKS_CLUSTER)" >/dev/null 2>&1; then \
		echo "EKS cluster $(EKS_CLUSTER) does not exist."; \
		exit 0; \
	fi; \
	$(MAKE) aws-kubeconfig >/dev/null; \
	echo; \
	echo "=== Nodes ==="; \
	$(KUBECTL_AWS) get nodes; \
	echo; \
	echo "=== Argo CD Applications ==="; \
	$(KUBECTL_AWS) get application -n argocd 2>/dev/null || true; \
	echo; \
	echo "=== Image Updater ==="; \
	$(KUBECTL_AWS) get imageupdater -n argocd 2>/dev/null || true; \
	echo; \
	echo "=== SignalDock Pods ==="; \
	$(KUBECTL_AWS) get pods -n signaldock 2>/dev/null || true; \
	echo; \
	echo "=== Persistent Volume Claims ==="; \
	$(KUBECTL_AWS) get pvc -n signaldock 2>/dev/null || true; \
	echo; \
	echo "=== Persistent Volumes ==="; \
	$(KUBECTL_AWS) get pv 2>/dev/null || true; \
	echo; \
	echo "=== External Secrets ==="; \
	$(KUBECTL_AWS) get externalsecret -n signaldock 2>/dev/null || true


# -----------------------------------------------------------------------------
# AWS teardown
# -----------------------------------------------------------------------------

aws-destroy:
	@set -euo pipefail; \
	echo "WARNING: this will permanently destroy the SignalDock AWS lab."; \
	echo "This includes EKS, EC2 nodes, EBS volumes, VPC resources and Secrets Manager secrets."; \
	echo; \
	printf "Type DESTROY to continue: "; \
	read -r answer; \
	if [ "$$answer" != "DESTROY" ]; then \
		echo "Aborted."; \
		exit 1; \
	fi; \
	echo; \
	echo "Checking AWS credentials..."; \
	aws sts get-caller-identity \
		--profile "$(AWS_PROFILE)" >/dev/null; \
	echo "Initializing Terraform..."; \
	AWS_PROFILE="$(AWS_PROFILE)" terraform \
		-chdir="$(TF_DIR)" \
		init >/dev/null; \
	pvs=""; \
	volumes=""; \
	if aws eks describe-cluster \
		--profile "$(AWS_PROFILE)" \
		--region "$(AWS_REGION)" \
		--name "$(EKS_CLUSTER)" >/dev/null 2>&1; then \
		echo "Selecting EKS cluster..."; \
		$(MAKE) aws-kubeconfig >/dev/null; \
		echo "Remembering SignalDock persistent volumes..."; \
		pvs="$$( \
			$(KUBECTL_AWS) get pvc \
				-n "$(HELM_NAMESPACE)" \
				-o jsonpath='{range .items[*]}{.spec.volumeName}{" "}{end}' \
				2>/dev/null || true \
		)"; \
		for pv in $$pvs; do \
			driver="$$( \
				$(KUBECTL_AWS) get pv "$$pv" \
					-o jsonpath='{.spec.csi.driver}' \
					2>/dev/null || true \
			)"; \
			if [ "$$driver" = "ebs.csi.aws.com" ]; then \
				volume="$$( \
					$(KUBECTL_AWS) get pv "$$pv" \
						-o jsonpath='{.spec.csi.volumeHandle}' \
						2>/dev/null || true \
				)"; \
				if [ -n "$$volume" ]; then \
					volumes="$$volumes $$volume"; \
					echo "  $$pv -> $$volume"; \
				fi; \
			fi; \
		done; \
		echo "Stopping App of Apps reconciliation..."; \
		if $(KUBECTL_AWS) get application "$(ARGO_ROOT_APPLICATION)" \
			-n argocd >/dev/null 2>&1; then \
			$(KUBECTL_AWS) patch application "$(ARGO_ROOT_APPLICATION)" \
				-n argocd \
				--type=merge \
				-p '{"metadata":{"finalizers":[]}}' >/dev/null; \
			$(KUBECTL_AWS) delete application "$(ARGO_ROOT_APPLICATION)" \
				-n argocd \
				--wait=true \
				--timeout=120s; \
		fi; \
		echo "Deleting GitOps applications..."; \
		for app in $(ARGO_CHILD_APPLICATIONS); do \
			if $(KUBECTL_AWS) get application "$$app" \
				-n argocd >/dev/null 2>&1; then \
				echo "  Deleting $$app..."; \
				$(KUBECTL_AWS) patch application "$$app" \
					-n argocd \
					--type=merge \
					-p '{"metadata":{"finalizers":["resources-finalizer.argocd.argoproj.io"]}}' \
					>/dev/null; \
				$(KUBECTL_AWS) delete application "$$app" \
					-n argocd \
					--wait=false; \
			fi; \
		done; \
		echo "Deleting SignalDock namespace and PVCs..."; \
		$(KUBECTL_AWS) delete namespace "$(HELM_NAMESPACE)" \
			--ignore-not-found=true \
			--wait=true \
			--timeout=300s; \
		echo "Waiting for GitOps applications to finish deleting..."; \
		for app in $(ARGO_CHILD_APPLICATIONS); do \
			if $(KUBECTL_AWS) get application "$$app" \
				-n argocd >/dev/null 2>&1; then \
				$(KUBECTL_AWS) wait \
					--for=delete \
					application/"$$app" \
					-n argocd \
					--timeout=300s; \
			fi; \
		done; \
		if [ -n "$$pvs" ]; then \
			echo "Waiting for SignalDock persistent volumes to disappear..."; \
			for i in $$(seq 1 60); do \
				remaining=""; \
				for pv in $$pvs; do \
					if $(KUBECTL_AWS) get pv "$$pv" \
						>/dev/null 2>&1; then \
						remaining="$$remaining $$pv"; \
					fi; \
				done; \
				[ -z "$$remaining" ] && break; \
				sleep 5; \
			done; \
			if [ -n "$$remaining" ]; then \
				echo "ERROR: persistent volumes still exist:$$remaining"; \
				echo "Terraform destroy has NOT been executed."; \
				exit 1; \
			fi; \
		fi; \
		if [ -n "$$volumes" ]; then \
			echo "Verifying dynamic EBS volumes in AWS..."; \
			remaining=""; \
			for i in $$(seq 1 60); do \
				remaining=""; \
				for volume in $$volumes; do \
					if aws ec2 describe-volumes \
						--profile "$(AWS_PROFILE)" \
						--region "$(AWS_REGION)" \
						--volume-ids "$$volume" \
						>/dev/null 2>&1; then \
						remaining="$$remaining $$volume"; \
					fi; \
				done; \
				[ -z "$$remaining" ] && break; \
				sleep 5; \
			done; \
			if [ -n "$$remaining" ]; then \
				echo "ERROR: dynamic EBS volumes still exist:$$remaining"; \
				echo "Terraform destroy has NOT been executed."; \
				exit 1; \
			fi; \
			echo "Dynamic EBS volumes deleted."; \
		fi; \
	else \
		echo "EKS cluster does not exist; skipping Kubernetes cleanup."; \
	fi; \
	echo; \
	echo "Destroying Terraform infrastructure..."; \
	AWS_PROFILE="$(AWS_PROFILE)" terraform \
		-chdir="$(TF_DIR)" \
		destroy \
		-auto-approve; \
	echo; \
	echo "SignalDock AWS lab destroyed."