# AWS Region
variable "aws_region" {
  description = "AWS region to deploy resources"
  type        = string
  default     = "us-west-2"
}

# Environment
variable "environment" {
  description = "Environment name (dev, staging, prod)"
  type        = string
  default     = "dev"

  validation {
    condition     = contains(["dev", "staging", "prod"], var.environment)
    error_message = "Environment must be dev, staging, or prod."
  }
}

# Project Name
variable "project_name" {
  description = "Project name for resource naming"
  type        = string
  default     = "streamspace"
}

# VPC Configuration
variable "vpc_cidr" {
  description = "CIDR block for VPC"
  type        = string
  default     = "10.0.0.0/16"
}

variable "availability_zones_count" {
  description = "Number of availability zones to use"
  type        = number
  default     = 3
}

# EKS Configuration
variable "kubernetes_version" {
  description = "Kubernetes version for EKS cluster"
  type        = string
  default     = "1.28"
}

variable "cluster_endpoint_public_access" {
  description = "Enable public access to EKS API endpoint"
  type        = bool
  default     = true
}

# System Node Group (Controller, Monitoring)
variable "system_instance_type" {
  description = "Instance type for system nodes"
  type        = string
  default     = "t3.large"
}

variable "system_min_size" {
  description = "Minimum number of system nodes"
  type        = number
  default     = 2
}

variable "system_max_size" {
  description = "Maximum number of system nodes"
  type        = number
  default     = 4
}

variable "system_desired_size" {
  description = "Desired number of system nodes"
  type        = number
  default     = 2
}

# Workload Node Group (User Sessions)
variable "workload_instance_type" {
  description = "Instance type for workload nodes"
  type        = string
  default     = "t3.xlarge"
}

variable "workload_capacity_type" {
  description = "Capacity type for workload nodes (ON_DEMAND or SPOT)"
  type        = string
  default     = "SPOT"

  validation {
    condition     = contains(["ON_DEMAND", "SPOT"], var.workload_capacity_type)
    error_message = "Capacity type must be ON_DEMAND or SPOT."
  }
}

variable "workload_min_size" {
  description = "Minimum number of workload nodes"
  type        = number
  default     = 1
}

variable "workload_max_size" {
  description = "Maximum number of workload nodes"
  type        = number
  default     = 20
}

variable "workload_desired_size" {
  description = "Desired number of workload nodes"
  type        = number
  default     = 2
}

# GPU Node Group (Optional)
variable "enable_gpu_nodes" {
  description = "Enable GPU node group"
  type        = bool
  default     = false
}

variable "gpu_instance_type" {
  description = "Instance type for GPU nodes"
  type        = string
  default     = "g4dn.xlarge"
}

variable "gpu_min_size" {
  description = "Minimum number of GPU nodes"
  type        = number
  default     = 0
}

variable "gpu_max_size" {
  description = "Maximum number of GPU nodes"
  type        = number
  default     = 5
}

variable "gpu_desired_size" {
  description = "Desired number of GPU nodes"
  type        = number
  default     = 0
}

# RDS Configuration
variable "enable_rds" {
  description = "Enable RDS PostgreSQL for StreamSpace API"
  type        = bool
  default     = true
}

variable "db_instance_class" {
  description = "RDS instance class"
  type        = string
  default     = "db.t3.medium"
}

variable "db_allocated_storage" {
  description = "Initial allocated storage for RDS (GB)"
  type        = number
  default     = 50
}

variable "db_max_allocated_storage" {
  description = "Maximum allocated storage for RDS autoscaling (GB)"
  type        = number
  default     = 200
}

# Tags
variable "tags" {
  description = "Additional tags for all resources"
  type        = map(string)
  default     = {}
}
