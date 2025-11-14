# VPC Outputs
output "vpc_id" {
  description = "VPC ID"
  value       = module.vpc.vpc_id
}

output "vpc_cidr" {
  description = "VPC CIDR block"
  value       = module.vpc.vpc_cidr_block
}

output "private_subnets" {
  description = "Private subnet IDs"
  value       = module.vpc.private_subnets
}

output "public_subnets" {
  description = "Public subnet IDs"
  value       = module.vpc.public_subnets
}

# EKS Outputs
output "cluster_id" {
  description = "EKS cluster ID"
  value       = module.eks.cluster_id
}

output "cluster_name" {
  description = "EKS cluster name"
  value       = module.eks.cluster_name
}

output "cluster_endpoint" {
  description = "EKS cluster endpoint"
  value       = module.eks.cluster_endpoint
  sensitive   = true
}

output "cluster_security_group_id" {
  description = "Security group ID attached to the EKS cluster"
  value       = module.eks.cluster_security_group_id
}

output "cluster_oidc_issuer_url" {
  description = "The URL on the EKS cluster OIDC Issuer"
  value       = module.eks.cluster_oidc_issuer_url
}

output "cluster_certificate_authority_data" {
  description = "Base64 encoded certificate data required to communicate with the cluster"
  value       = module.eks.cluster_certificate_authority_data
  sensitive   = true
}

# IAM Role ARNs
output "ebs_csi_controller_role_arn" {
  description = "ARN of IAM role for EBS CSI controller"
  value       = module.ebs_csi_irsa.iam_role_arn
}

output "efs_csi_controller_role_arn" {
  description = "ARN of IAM role for EFS CSI controller"
  value       = module.efs_csi_irsa.iam_role_arn
}

output "cluster_autoscaler_role_arn" {
  description = "ARN of IAM role for Cluster Autoscaler"
  value       = module.cluster_autoscaler_irsa.iam_role_arn
}

output "load_balancer_controller_role_arn" {
  description = "ARN of IAM role for AWS Load Balancer Controller"
  value       = module.load_balancer_controller_irsa.iam_role_arn
}

# EFS Outputs
output "efs_id" {
  description = "EFS file system ID"
  value       = aws_efs_file_system.streamspace.id
}

output "efs_dns_name" {
  description = "EFS DNS name"
  value       = aws_efs_file_system.streamspace.dns_name
}

# RDS Outputs (if enabled)
output "db_endpoint" {
  description = "RDS PostgreSQL endpoint"
  value       = var.enable_rds ? module.db[0].db_instance_endpoint : null
  sensitive   = true
}

output "db_name" {
  description = "RDS database name"
  value       = var.enable_rds ? module.db[0].db_instance_name : null
}

output "db_username" {
  description = "RDS master username"
  value       = var.enable_rds ? module.db[0].db_instance_username : null
  sensitive   = true
}

output "db_password" {
  description = "RDS master password (managed by AWS Secrets Manager)"
  value       = var.enable_rds ? module.db[0].db_instance_password : null
  sensitive   = true
}

# Configuration Commands
output "configure_kubectl" {
  description = "Command to configure kubectl"
  value       = "aws eks update-kubeconfig --region ${var.aws_region} --name ${module.eks.cluster_name}"
}

output "streamspace_helm_values" {
  description = "Helm values for StreamSpace deployment on AWS"
  value       = <<-EOT
    # Use these values for deploying StreamSpace on AWS EKS

    global:
      storageClass: efs-sc  # Use EFS storage class

    api:
      auth:
        mode: hybrid

      ${var.enable_rds ? "postgresql:" : "# PostgreSQL (using internal deployment)"}
      ${var.enable_rds ? "  external:" : ""}
      ${var.enable_rds ? "    enabled: true" : ""}
      ${var.enable_rds ? "    host: ${try(module.db[0].db_instance_endpoint, "localhost")}" : ""}
      ${var.enable_rds ? "    port: 5432" : ""}
      ${var.enable_rds ? "    database: streamspace" : ""}
      ${var.enable_rds ? "    username: streamspace" : ""}
      ${var.enable_rds ? "    existingSecret: streamspace-db-credentials" : ""}

    controller:
      nodeSelector:
        streamspace.io/node-type: system
      tolerations:
        - key: streamspace.io/system
          operator: Equal
          value: "true"
          effect: NoSchedule

    ingress:
      enabled: true
      className: alb  # Use AWS Load Balancer Controller
      annotations:
        alb.ingress.kubernetes.io/scheme: internet-facing
        alb.ingress.kubernetes.io/target-type: ip
        alb.ingress.kubernetes.io/certificate-arn: arn:aws:acm:REGION:ACCOUNT:certificate/CERT_ID
    EOT
}
