# StreamSpace AWS Deployment Guide

This guide walks you through deploying StreamSpace on Amazon Web Services (AWS) using Amazon EKS (Elastic Kubernetes Service).

## Overview

StreamSpace on AWS provides:
- **Fully managed Kubernetes** with Amazon EKS
- **Auto-scaling** node groups with Cluster Autoscaler
- **Persistent storage** with Amazon EFS
- **Managed database** with Amazon RDS PostgreSQL (optional)
- **Load balancing** with AWS Load Balancer Controller
- **Cost optimization** with Spot instances for workloads
- **Multi-AZ deployment** for high availability
- **GPU support** for graphics-intensive sessions (optional)

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         AWS Cloud                                │
│                                                                   │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │                    VPC (Multi-AZ)                           │ │
│  │                                                             │ │
│  │  ┌──────────────┐     ┌──────────────┐     ┌────────────┐ │ │
│  │  │  Public      │     │  Public      │     │  Public    │ │ │
│  │  │  Subnet      │     │  Subnet      │     │  Subnet    │ │ │
│  │  │  (AZ-1)      │     │  (AZ-2)      │     │  (AZ-3)    │ │ │
│  │  │  - NAT GW    │     │  - NAT GW    │     │  - NAT GW  │ │ │
│  │  │  - ALB       │     │              │     │            │ │ │
│  │  └──────┬───────┘     └──────┬───────┘     └──────┬─────┘ │ │
│  │         │                    │                    │       │ │
│  │  ┌──────┴───────┐     ┌──────┴───────┐     ┌──────┴─────┐ │ │
│  │  │  Private     │     │  Private     │     │  Private   │ │ │
│  │  │  Subnet      │     │  Subnet      │     │  Subnet    │ │ │
│  │  │  (AZ-1)      │     │  (AZ-2)      │     │  (AZ-3)    │ │ │
│  │  │              │     │              │     │            │ │ │
│  │  │  EKS Nodes:  │     │  EKS Nodes:  │     │  EKS Nodes:│ │ │
│  │  │  - System    │     │  - System    │     │  - System  │ │ │
│  │  │  - Workload  │     │  - Workload  │     │  - Workload│ │ │
│  │  │  - GPU (opt) │     │  - GPU (opt) │     │  - GPU     │ │ │
│  │  └──────────────┘     └──────────────┘     └────────────┘ │ │
│  │                                                             │ │
│  │  Storage:                Database:                          │ │
│  │  - EFS (multi-AZ)       - RDS PostgreSQL (multi-AZ)        │ │
│  └─────────────────────────────────────────────────────────────┘ │
│                                                                   │
│  Management:                                                      │
│  - EKS Control Plane        - Auto Scaling Groups                │
│  - KMS Encryption           - CloudWatch Monitoring              │
└───────────────────────────────────────────────────────────────────┘
```

## Prerequisites

### 1. AWS Account
- Active AWS account with appropriate permissions
- IAM user or role with permissions for:
  - EC2, VPC, EKS, EFS, RDS
  - IAM role creation
  - KMS key management

### 2. Tools Installation
```bash
# AWS CLI
brew install awscli  # macOS
# OR
curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip"
unzip awscliv2.zip
sudo ./aws/install

# Terraform
brew install terraform  # macOS
# OR
wget https://releases.hashicorp.com/terraform/1.6.0/terraform_1.6.0_linux_amd64.zip
unzip terraform_1.6.0_linux_amd64.zip
sudo mv terraform /usr/local/bin/

# kubectl
brew install kubectl  # macOS
# OR
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
chmod +x kubectl
sudo mv kubectl /usr/local/bin/

# Helm
brew install helm  # macOS
# OR
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
```

### 3. Configure AWS CLI
```bash
aws configure
# Enter:
# - AWS Access Key ID
# - AWS Secret Access Key
# - Default region (e.g., us-west-2)
# - Default output format (json)

# Verify configuration
aws sts get-caller-identity
```

## Quick Start

### 1. Clone Repository
```bash
git clone https://github.com/streamspace/streamspace.git
cd streamspace/terraform/aws
```

### 2. Configure Variables
Create `terraform.tfvars`:

```hcl
# Basic configuration
aws_region   = "us-west-2"
environment  = "prod"
project_name = "streamspace"

# VPC
vpc_cidr = "10.0.0.0/16"
availability_zones_count = 3

# EKS
kubernetes_version = "1.28"

# System nodes (controller, monitoring)
system_instance_type = "t3.large"
system_min_size      = 2
system_max_size      = 4
system_desired_size  = 2

# Workload nodes (user sessions)
workload_instance_type  = "t3.xlarge"
workload_capacity_type  = "SPOT"  # Use Spot for cost savings
workload_min_size       = 2
workload_max_size       = 20
workload_desired_size   = 3

# GPU nodes (optional)
enable_gpu_nodes    = false
gpu_instance_type   = "g4dn.xlarge"
gpu_min_size        = 0
gpu_max_size        = 5
gpu_desired_size    = 0

# Database
enable_rds             = true
db_instance_class      = "db.t3.medium"
db_allocated_storage   = 50
db_max_allocated_storage = 200
```

### 3. Deploy Infrastructure
```bash
# Initialize Terraform
terraform init

# Review the execution plan
terraform plan

# Apply the configuration
terraform apply

# This will take 15-20 minutes to create:
# - VPC with 3 AZs
# - EKS cluster
# - Node groups (system + workload)
# - EFS file system
# - RDS PostgreSQL (if enabled)
# - IAM roles and policies
# - Security groups
```

### 4. Configure kubectl
```bash
# Get the command from Terraform output
aws eks update-kubeconfig --region us-west-2 --name streamspace-prod

# Verify connection
kubectl get nodes
```

### 5. Install Cluster Add-ons

#### a. AWS Load Balancer Controller
```bash
# Add Helm repository
helm repo add eks https://aws.github.io/eks-charts
helm repo update

# Install controller
helm install aws-load-balancer-controller eks/aws-load-balancer-controller \
  -n kube-system \
  --set clusterName=streamspace-prod \
  --set serviceAccount.create=false \
  --set serviceAccount.name=aws-load-balancer-controller
```

#### b. EFS CSI Driver
```bash
# Already installed via Terraform, verify:
kubectl get pods -n kube-system | grep efs-csi
```

#### c. Cluster Autoscaler
```bash
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cluster-autoscaler
  namespace: kube-system
  labels:
    app: cluster-autoscaler
spec:
  replicas: 1
  selector:
    matchLabels:
      app: cluster-autoscaler
  template:
    metadata:
      labels:
        app: cluster-autoscaler
    spec:
      serviceAccountName: cluster-autoscaler
      containers:
      - image: k8s.gcr.io/autoscaling/cluster-autoscaler:v1.28.0
        name: cluster-autoscaler
        resources:
          limits:
            cpu: 100m
            memory: 600Mi
          requests:
            cpu: 100m
            memory: 600Mi
        command:
          - ./cluster-autoscaler
          - --v=4
          - --stderrthreshold=info
          - --cloud-provider=aws
          - --skip-nodes-with-local-storage=false
          - --expander=least-waste
          - --node-group-auto-discovery=asg:tag=k8s.io/cluster-autoscaler/enabled,k8s.io/cluster-autoscaler/streamspace-prod
        env:
          - name: AWS_REGION
            value: us-west-2
EOF
```

### 6. Create EFS Storage Class
```bash
# Get EFS ID from Terraform output
EFS_ID=$(terraform output -raw efs_id)

# Create storage class
kubectl apply -f - <<EOF
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: efs-sc
provisioner: efs.csi.aws.com
parameters:
  provisioningMode: efs-ap
  fileSystemId: ${EFS_ID}
  directoryPerms: "700"
  gidRangeStart: "1000"
  gidRangeEnd: "2000"
EOF
```

### 7. Deploy StreamSpace

#### a. Create Database Secret (if using RDS)
```bash
# Get RDS credentials from Terraform output
DB_PASSWORD=$(terraform output -raw db_password)

kubectl create secret generic streamspace-db-credentials \
  --from-literal=password="${DB_PASSWORD}" \
  -n streamspace
```

#### b. Create TLS Certificate (optional)
```bash
# Request certificate in AWS Certificate Manager
aws acm request-certificate \
  --domain-name streamspace.example.com \
  --validation-method DNS \
  --region us-west-2

# Note the certificate ARN for ingress configuration
```

#### c. Install StreamSpace with Helm
```bash
# Get RDS endpoint
DB_ENDPOINT=$(terraform output -raw db_endpoint)

# Create values file
cat > aws-values.yaml <<EOF
global:
  storageClass: efs-sc

controller:
  nodeSelector:
    streamspace.io/node-type: system
  tolerations:
    - key: streamspace.io/system
      operator: Equal
      value: "true"
      effect: NoSchedule

api:
  replicaCount: 3

  auth:
    mode: hybrid

  postgresql:
    enabled: false  # Using external RDS
    external:
      enabled: true
      host: ${DB_ENDPOINT%%:*}
      port: 5432
      database: streamspace
      username: streamspace
      existingSecret: streamspace-db-credentials

  nodeSelector:
    streamspace.io/node-type: system
  tolerations:
    - key: streamspace.io/system
      operator: Equal
      value: "true"
      effect: NoSchedule

ui:
  replicaCount: 3

  nodeSelector:
    streamspace.io/node-type: system
  tolerations:
    - key: streamspace.io/system
      operator: Equal
      value: "true"
      effect: NoSchedule

ingress:
  enabled: true
  className: alb
  annotations:
    alb.ingress.kubernetes.io/scheme: internet-facing
    alb.ingress.kubernetes.io/target-type: ip
    alb.ingress.kubernetes.io/certificate-arn: arn:aws:acm:REGION:ACCOUNT:certificate/CERT_ID
    alb.ingress.kubernetes.io/listen-ports: '[{"HTTP": 80}, {"HTTPS": 443}]'
    alb.ingress.kubernetes.io/ssl-redirect: "443"
  hosts:
    - host: streamspace.example.com
      paths:
        - path: /
          pathType: Prefix
          service: ui
        - path: /api
          pathType: Prefix
          service: api
  tls:
    enabled: true

sessionDefaults:
  resources:
    requests:
      memory: 4Gi
      cpu: 2000m
    limits:
      memory: 8Gi
      cpu: 4000m
  persistentHome:
    enabled: true
    size: 100Gi
    storageClass: efs-sc

monitoring:
  enabled: true
EOF

# Install StreamSpace
helm install streamspace ../../chart \
  --namespace streamspace \
  --create-namespace \
  --values aws-values.yaml
```

### 8. Verify Deployment
```bash
# Check pods
kubectl get pods -n streamspace

# Check ingress
kubectl get ingress -n streamspace

# Get ALB DNS name
kubectl get ingress streamspace -n streamspace \
  -o jsonpath='{.status.loadBalancer.ingress[0].hostname}'

# Check cluster stats
kubectl get nodes -o wide
```

## Cost Optimization

### 1. Use Spot Instances
Workload nodes use Spot instances by default (60-90% cost savings):
```hcl
workload_capacity_type = "SPOT"
```

### 2. Auto-scaling
Cluster Autoscaler automatically adjusts node count based on demand:
- Scales down idle nodes after 10 minutes
- Scales up when pods are pending

### 3. Right-size Resources
Monitor resource usage and adjust node instance types:
```bash
# View node resource usage
kubectl top nodes

# Adjust in terraform.tfvars
workload_instance_type = "t3.large"  # Start smaller
```

### 4. EFS Lifecycle Policies
Inactive files transition to Infrequent Access (IA) after 30 days (configured in Terraform).

### 5. RDS Reserved Instances
For production, purchase RDS Reserved Instances for 40-60% savings:
```bash
aws rds purchase-reserved-db-instances-offering \
  --reserved-db-instances-offering-id <offering-id> \
  --reserved-db-instance-id streamspace-prod-rds-ri
```

## Scaling

### Horizontal Scaling

**Automatic (Cluster Autoscaler)**:
- Node groups scale based on pending pods
- Min/max configured in Terraform variables

**Manual**:
```bash
# Scale workload node group
aws eks update-nodegroup-config \
  --cluster-name streamspace-prod \
  --nodegroup-name streamspace-prod-workload \
  --scaling-config minSize=5,maxSize=30,desiredSize=10
```

### Vertical Scaling

Change instance types:
```hcl
# In terraform.tfvars
workload_instance_type = "t3.2xlarge"  # Upgrade

# Apply changes
terraform apply
```

## Monitoring

### CloudWatch
```bash
# View EKS cluster metrics
aws cloudwatch get-metric-statistics \
  --namespace AWS/EKS \
  --metric-name cluster_node_count \
  --dimensions Name=ClusterName,Value=streamspace-prod \
  --start-time 2024-01-01T00:00:00Z \
  --end-time 2024-01-02T00:00:00Z \
  --period 3600 \
  --statistics Average
```

### Prometheus & Grafana
StreamSpace includes built-in monitoring (if enabled):
```bash
# Port-forward Grafana
kubectl port-forward -n streamspace svc/streamspace-grafana 3000:80

# Access: http://localhost:3000
```

## Backup & Disaster Recovery

### EFS Backups
```bash
# Enable automatic backups
aws backup create-backup-plan \
  --backup-plan file://backup-plan.json
```

### RDS Automated Backups
- Configured via Terraform (30 days retention for prod)
- Point-in-time recovery available

### Database Manual Snapshot
```bash
aws rds create-db-snapshot \
  --db-instance-identifier streamspace-prod-db \
  --db-snapshot-identifier streamspace-backup-$(date +%Y%m%d)
```

## Security

### Network Security
- Private subnets for EKS nodes
- Security groups restrict traffic
- NAT gateways for outbound traffic

### Encryption
- EKS secrets encrypted with KMS
- EFS encrypted at rest
- RDS encrypted at rest
- TLS for ingress traffic

### IAM Roles
- Pod-level IAM roles via IRSA
- Principle of least privilege

### Security Scanning
```bash
# Scan images with Trivy
trivy image ghcr.io/streamspace/streamspace-api:latest
```

## Troubleshooting

### Pods Not Scheduling
```bash
# Check node capacity
kubectl describe nodes | grep -A 5 "Allocated resources"

# Check for taints
kubectl get nodes -o json | jq '.items[].spec.taints'

# View pending pods
kubectl get pods -n streamspace --field-selector=status.phase=Pending
```

### EFS Mount Issues
```bash
# Check EFS CSI driver
kubectl get pods -n kube-system | grep efs-csi

# View CSI driver logs
kubectl logs -n kube-system -l app=efs-csi-controller
```

### RDS Connection Failed
```bash
# Check security group
aws ec2 describe-security-groups \
  --group-ids $(terraform output -raw db_security_group_id)

# Test connectivity from pod
kubectl run -it --rm debug --image=postgres:15 -- \
  psql -h <RDS_ENDPOINT> -U streamspace -d streamspace
```

### Load Balancer Not Created
```bash
# Check ALB controller
kubectl logs -n kube-system -l app.kubernetes.io/name=aws-load-balancer-controller

# View ingress events
kubectl describe ingress streamspace -n streamspace
```

## Cleanup

### Destroy Infrastructure
```bash
# Uninstall StreamSpace first
helm uninstall streamspace -n streamspace

# Delete any PVCs
kubectl delete pvc --all -n streamspace

# Destroy Terraform resources
terraform destroy

# This will delete:
# - EKS cluster and node groups
# - VPC and subnets
# - EFS file system
# - RDS database (if not protected)
# - IAM roles
# - Security groups
```

**Warning**: This is irreversible. Ensure you have backups before destroying.

## Support

- **Documentation**: https://docs.streamspace.io/aws
- **AWS Support**: https://console.aws.amazon.com/support
- **GitHub Issues**: https://github.com/streamspace/streamspace/issues

## Cost Estimate

Approximate monthly costs for different deployment sizes:

### Small (Dev/Testing)
- 2x t3.large system nodes: $120
- 2x t3.xlarge spot workload nodes: $60
- EFS (100GB): $30
- RDS db.t3.small: $30
- **Total**: ~$240/month

### Medium (Production)
- 3x t3.large system nodes: $180
- 5x t3.xlarge spot workload nodes: $150
- EFS (500GB): $150
- RDS db.t3.medium (multi-AZ): $120
- ALB: $25
- **Total**: ~$625/month

### Large (Enterprise)
- 3x t3.2xlarge system nodes: $360
- 10x t3.2xlarge spot workload nodes: $600
- 3x g4dn.xlarge GPU nodes: $450
- EFS (2TB): $600
- RDS db.r5.xlarge (multi-AZ): $600
- ALB + WAF: $50
- **Total**: ~$2,660/month

*Costs are estimates and vary by region and usage*
