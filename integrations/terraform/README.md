# Go AI POI Kubernetes Infrastructure

This Terraform configuration deploys a complete Kubernetes cluster for the Go AI POI application with full observability stack including Prometheus, Loki, Tempo, and Grafana.

## Architecture

The infrastructure includes:

- **Application Namespace (`loci-server`):**
  - Go AI POI application deployment
  - PostgreSQL database with persistent storage
  - Application secrets and configuration

- **Observability Namespace (`observability`):**
  - Prometheus for metrics collection
  - Loki for log aggregation
  - Tempo for distributed tracing
  - OpenTelemetry Collector for telemetry processing
  - Grafana for visualization and dashboards

- **Ingress:**
  - NGINX Ingress Controller
  - Application and observability service exposure

## Prerequisites

1. **Kubernetes Cluster**: Have a running Kubernetes cluster (local or cloud)
2. **kubectl**: Configured to connect to your cluster
3. **Terraform**: Version >= 1.0
4. **Helm**: For installing NGINX Ingress Controller
5. **k9s**: For debugging Kubernetes resources (optional but recommended)

## Setup Commands (Step by Step)

### Step 1: Setup Kubernetes Cluster

**For local development (choose one):**

**Option A: minikube**
```bash
# Install minikube (if not already installed)
# For macOS: brew install minikube
# For Linux: curl -LO https://storage.googleapis.com/minikube/releases/latest/minikube-linux-amd64

# Start minikube
minikube start --driver=docker --memory=4096 --cpus=2

# Enable ingress addon
minikube addons enable ingress

# Verify cluster is running
kubectl cluster-info
kubectl get nodes
```

**Option B: kind (Kubernetes in Docker)**
```bash
# Install kind (if not already installed)
# For macOS: brew install kind
# For Linux: GO111MODULE="on" go install sigs.k8s.io/kind@v0.20.0

# Create cluster with ingress support
cat <<EOF | kind create cluster --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  - containerPort: 80
    hostPort: 80
    protocol: TCP
  - containerPort: 443
    hostPort: 443
    protocol: TCP
EOF

# Install NGINX Ingress Controller
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml

# Wait for ingress controller to be ready
kubectl wait --namespace ingress-nginx --for=condition=ready pod --selector=app.kubernetes.io/component=controller --timeout=90s
```

### Step 2: Install Required Tools

```bash
# Install helm (if not installed)
# For macOS: brew install helm
# For Linux: curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

# Install k9s for debugging (optional but recommended)
# For macOS: brew install k9s
# For Linux: curl -sS https://webinstall.dev/k9s | bash

# Verify installations
helm version
k9s version
kubectl version --client
terraform version
```

### Step 3: Prepare Docker Image

```bash
# Navigate to project root (not terraform directory)
cd ..

# Build the Docker image
docker build -t loci-server:latest .

# Load image into your Kubernetes cluster
# For minikube:
minikube image load loci-server:latest

# For kind:
kind load docker-image loci-server:latest

# Verify image is loaded
# For minikube:
minikube image ls | grep loci-server
# For kind:
docker exec -it kind-control-plane crictl images | grep loci-server
```

### Step 4: Configure Terraform

```bash
# Navigate to terraform directory
cd terraform

# Copy and customize variables
cp terraform.tfvars.example terraform.tfvars

# Edit terraform.tfvars with your preferred editor
# Update these key values:
# - app_version = "latest"
# - postgres_password = "your-secure-password"
# - jwt_secret = "your-jwt-secret-key"
# - domain = "loci-server.local" (for local development)
```

### Step 5: Deploy Infrastructure

```bash
# Initialize Terraform
terraform init

# Validate configuration
terraform validate

# Plan the deployment (review what will be created)
terraform plan

# Apply the configuration
terraform apply
# Type 'yes' when prompted

# Wait for deployment to complete (usually 2-3 minutes)
```

### Step 6: Verify Deployment

```bash
# Check all namespaces
kubectl get namespaces

# Check pods in application namespace
kubectl get pods -n loci-server

# Check pods in observability namespace
kubectl get pods -n observability

# Check services
kubectl get svc -n loci-server
kubectl get svc -n observability

# Check ingress
kubectl get ingress -A

# Wait for all pods to be ready (this may take a few minutes)
kubectl wait --for=condition=ready pod --all -n loci-server --timeout=300s
kubectl wait --for=condition=ready pod --all -n observability --timeout=300s
```

### Step 7: (Alternative) Using Helm for Deployment

Instead of Terraform, you can use Helm charts for more flexibility:

```bash
# Add required Helm repositories
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo add grafana https://grafana.github.io/helm-charts
helm repo update

# Create namespace
kubectl create namespace loci-server

# Install using Helm
cd helm/loci-server

# Customize values (optional)
cp values.yaml values-local.yaml
# Edit values-local.yaml with your configurations

# Install the chart
helm install loci-server . --namespace loci-server --values values-local.yaml

# Verify installation
helm list -n loci-server
kubectl get pods -n loci-server

# Upgrade chart (after making changes)
helm upgrade loci-server . --namespace loci-server --values values-local.yaml

# Uninstall
helm uninstall loci-server --namespace loci-server
```

### Step 8: (Advanced) GitOps with ArgoCD

For production environments, use ArgoCD for GitOps deployment:

#### Install ArgoCD

```bash
# Create ArgoCD namespace
kubectl create namespace argocd

# Install ArgoCD
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml

# Apply custom configuration
kubectl apply -f argocd/install/argocd-install.yaml

# Wait for ArgoCD to be ready
kubectl wait --for=condition=ready pod --all -n argocd --timeout=300s

# Get initial admin password
kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d; echo

# Port forward to access ArgoCD UI (if not using ingress)
kubectl port-forward svc/argocd-server -n argocd 8080:443

# Access ArgoCD UI at: https://localhost:8080 or https://argocd.loci-server.local
# Username: admin
# Password: (from command above)
```

#### Deploy Applications with ArgoCD

```bash
# Update the repository URL in ArgoCD manifests
# Edit argocd/applications/loci-server-app.yaml and argocd/applications/loci-server-observability.yaml
# Replace "https://github.com/your-org/loci-server.git" with your actual repository URL

# Apply ArgoCD project
kubectl apply -f argocd/projects/loci-server-project.yaml

# Apply applications
kubectl apply -f argocd/applications/loci-server-app.yaml
kubectl apply -f argocd/applications/loci-server-observability.yaml

# Check application status
kubectl get applications -n argocd

# Sync applications (if not auto-synced)
argocd app sync loci-server
argocd app sync loci-server-observability
```

#### ArgoCD CLI Setup (Optional)

```bash
# Install ArgoCD CLI
# For macOS: brew install argocd
# For Linux: curl -sSL -o argocd-linux-amd64 https://github.com/argoproj/argo-cd/releases/latest/download/argocd-linux-amd64

# Login to ArgoCD
argocd login argocd.loci-server.local  # or localhost:8080
# Username: admin
# Password: (from initial secret)

# List applications
argocd app list

# Get application details
argocd app get loci-server

# Sync application
argocd app sync loci-server

# View application logs
argocd app logs loci-server
```

## Configuration

### Key Variables (terraform.tfvars)

```hcl
# Application
app_name     = "loci-server"
app_version  = "latest"  # Use your Docker image tag
app_replicas = 2

# Database
postgres_password = "secure-password"

# Security
jwt_secret = "your-secure-jwt-secret"

# Networking
domain = "your-domain.com"  # or "localhost" for local testing
enable_ingress = true
```

## Accessing Services

After deployment, you can access:

- **Application**: `http://your-domain.com`
- **Grafana**: `http://grafana.your-domain.com` (admin/admin123)
- **Prometheus**: `http://prometheus.your-domain.com`

For local development, add to `/etc/hosts`:
```
127.0.0.1 loci-server.local
127.0.0.1 grafana.loci-server.local
127.0.0.1 prometheus.loci-server.local
```

## Port Forwarding (Alternative to Ingress)

If you disable ingress (`enable_ingress = false`), use port forwarding:

```bash
# Application
kubectl port-forward -n loci-server svc/loci-server-service 8000:8000

# Grafana
kubectl port-forward -n observability svc/grafana-service 3000:3000

# Prometheus
kubectl port-forward -n observability svc/prometheus-service 9090:9090
```

## Docker Image

Before deploying, ensure your Docker image is built and available:

```bash
# Build the image
docker build -t loci-server:latest .

# For local Kubernetes (minikube/kind), load the image:
# minikube image load loci-server:latest
# kind load docker-image loci-server:latest
```

## Monitoring and Observability

The stack provides comprehensive observability:

1. **Metrics**: Prometheus scrapes metrics from your application
2. **Logs**: Loki collects logs via OpenTelemetry Collector
3. **Traces**: Tempo receives traces via OTLP
4. **Dashboards**: Grafana provides unified visualization

### Application Instrumentation

Ensure your Go application exports:
- Metrics on port 8084 at `/metrics`
- OTLP traces to `otel-collector-service.observability.svc.cluster.local:4317`
- Structured logs compatible with Loki

## Persistence

Persistent volumes are created for:
- PostgreSQL data (`10Gi`)
- Prometheus data (`5Gi`)
- Loki data (`5Gi`)
- Tempo traces (`5Gi`)
- Grafana dashboards (`2Gi`)

## Scaling

Scale application replicas:
```bash
kubectl scale deployment loci-server -n loci-server --replicas=5
```

Or update `app_replicas` in terraform.tfvars and re-apply.

## Cleanup

```bash
terraform destroy
```

## Debugging Commands

### Using kubectl for Basic Debugging

```bash
# Check pod status and details
kubectl get pods -n loci-server -o wide
kubectl get pods -n observability -o wide

# Describe pods for detailed info
kubectl describe pod <pod-name> -n loci-server
kubectl describe pod <pod-name> -n observability

# View pod logs
kubectl logs -n loci-server deployment/loci-server
kubectl logs -n loci-server deployment/loci-server --previous  # Previous container logs
kubectl logs -n loci-server deployment/loci-server -f  # Follow logs

# View logs for specific pods
kubectl logs -n observability deployment/prometheus
kubectl logs -n observability deployment/loki
kubectl logs -n observability deployment/tempo
kubectl logs -n observability deployment/grafana

# Check services and endpoints
kubectl get svc -n loci-server
kubectl get svc -n observability
kubectl get endpoints -n loci-server
kubectl get endpoints -n observability

# Check ingress
kubectl get ingress -A
kubectl describe ingress loci-server-ingress -n loci-server

# Check persistent volumes
kubectl get pv
kubectl get pvc -n loci-server
kubectl get pvc -n observability

# Execute commands inside pods
kubectl exec -it deployment/loci-server -n loci-server -- /bin/sh
kubectl exec -it deployment/postgresql -n loci-server -- psql -U postgres

# Port forward for direct access (alternative to ingress)
kubectl port-forward -n loci-server svc/loci-server-service 8000:8000
kubectl port-forward -n observability svc/grafana-service 3000:3000
kubectl port-forward -n observability svc/prometheus-service 9090:9090
```

### Using k9s for Advanced Debugging

```bash
# Launch k9s
k9s

# Key k9s commands once inside:
# :namespaces or :ns     - View all namespaces
# :pods                  - View all pods across namespaces
# :pods -n loci-server     - View pods in specific namespace
# :svc                   - View all services
# :ing                   - View all ingress resources
# :pv                    - View persistent volumes
# :pvc                   - View persistent volume claims
# :events                - View cluster events
# :top pods              - View pod resource usage
# :top nodes             - View node resource usage
# :applications          - View ArgoCD applications (if ArgoCD is installed)

# Navigation within k9s:
# Enter         - Describe selected resource
# l             - View logs of selected pod
# s             - Shell into selected pod
# e             - Edit selected resource
# d             - Delete selected resource
# f             - Port forward
# y             - View YAML of selected resource
# ?             - Show help
# Ctrl+c        - Exit k9s
```

### Helm-specific Debugging

```bash
# List Helm releases
helm list -A

# Get release status
helm status loci-server -n loci-server

# Get release history
helm history loci-server -n loci-server

# Debug Helm templates (dry-run)
helm install loci-server ./helm/loci-server --namespace loci-server --dry-run --debug

# Rollback to previous version
helm rollback loci-server 1 -n loci-server

# Test Helm hooks
helm test loci-server -n loci-server
```

### ArgoCD-specific Debugging

```bash
# Check ArgoCD applications
kubectl get applications -n argocd

# Describe application for detailed status
kubectl describe application loci-server -n argocd

# View ArgoCD application controller logs
kubectl logs -n argocd deployment/argocd-application-controller

# View ArgoCD server logs
kubectl logs -n argocd deployment/argocd-server

# Manual sync application
argocd app sync loci-server --force

# View application sync history
argocd app history loci-server

# Compare live vs desired state
argocd app diff loci-server
```

### Common Issues and Solutions

```bash
# If pods are stuck in Pending state
kubectl describe pod <pod-name> -n <namespace>
# Check for resource constraints or PVC issues

# If application can't connect to database
kubectl get svc postgresql -n loci-server
kubectl logs deployment/loci-server -n loci-server | grep -i error

# If ingress is not working
kubectl get ingress -A
kubectl describe ingress loci-server-ingress -n loci-server
kubectl logs -n ingress-nginx deployment/ingress-nginx-controller

# Check cluster resource usage
kubectl top nodes
kubectl top pods -A

# View recent cluster events
kubectl get events --sort-by=.metadata.creationTimestamp -A

# Restart deployment if needed
kubectl rollout restart deployment/loci-server -n loci-server
kubectl rollout restart deployment/postgresql -n loci-server
```

### Monitoring Resource Health

```bash
# Check overall cluster health
kubectl get componentstatuses
kubectl cluster-info dump

# Monitor deployment rollout
kubectl rollout status deployment/loci-server -n loci-server
kubectl rollout history deployment/loci-server -n loci-server

# Check resource quotas and limits
kubectl describe namespace loci-server
kubectl describe namespace observability

# View node conditions
kubectl describe nodes
```

## Security Notes

- Change default passwords in production
- Use proper secrets management (e.g., HashiCorp Vault)
- Enable TLS for ingress in production
- Review RBAC permissions
- Secure persistent volume access