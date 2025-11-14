# StreamSpace AWS Infrastructure
#
# This Terraform configuration deploys a production-ready EKS cluster
# for running StreamSpace with autoscaling, monitoring, and multi-AZ support.

terraform {
  required_version = ">= 1.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.20"
    }
    helm = {
      source  = "hashicorp/helm"
      version = "~> 2.10"
    }
  }

  # Backend configuration for state storage
  backend "s3" {
    # Uncomment and configure for remote state
    # bucket = "your-terraform-state-bucket"
    # key    = "streamspace/terraform.tfstate"
    # region = "us-west-2"
    # dynamodb_table = "terraform-state-lock"
    # encrypt = true
  }
}

# Provider configurations
provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      Environment = var.environment
      Project     = "StreamSpace"
      ManagedBy   = "Terraform"
    }
  }
}

provider "kubernetes" {
  host                   = module.eks.cluster_endpoint
  cluster_ca_certificate = base64decode(module.eks.cluster_certificate_authority_data)

  exec {
    api_version = "client.authentication.k8s.io/v1beta1"
    command     = "aws"
    args = [
      "eks",
      "get-token",
      "--cluster-name",
      module.eks.cluster_name,
    ]
  }
}

provider "helm" {
  kubernetes {
    host                   = module.eks.cluster_endpoint
    cluster_ca_certificate = base64decode(module.eks.cluster_certificate_authority_data)

    exec {
      api_version = "client.authentication.k8s.io/v1beta1"
      command     = "aws"
      args = [
        "eks",
        "get-token",
        "--cluster-name",
        module.eks.cluster_name,
      ]
    }
  }
}

# Data sources
data "aws_availability_zones" "available" {
  state = "available"
}

data "aws_caller_identity" "current" {}

# Local variables
locals {
  cluster_name = "${var.project_name}-${var.environment}"

  azs = slice(data.aws_availability_zones.available.names, 0, var.availability_zones_count)

  tags = {
    Environment = var.environment
    Project     = var.project_name
    Terraform   = "true"
  }
}

# VPC Module
module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "~> 5.0"

  name = "${local.cluster_name}-vpc"
  cidr = var.vpc_cidr

  azs             = local.azs
  private_subnets = [for k, v in local.azs : cidrsubnet(var.vpc_cidr, 4, k)]
  public_subnets  = [for k, v in local.azs : cidrsubnet(var.vpc_cidr, 8, k + 48)]

  enable_nat_gateway   = true
  single_nat_gateway   = var.environment == "dev" ? true : false
  enable_dns_hostnames = true
  enable_dns_support   = true

  # Kubernetes tags
  public_subnet_tags = {
    "kubernetes.io/role/elb"                    = "1"
    "kubernetes.io/cluster/${local.cluster_name}" = "shared"
  }

  private_subnet_tags = {
    "kubernetes.io/role/internal-elb"           = "1"
    "kubernetes.io/cluster/${local.cluster_name}" = "shared"
  }

  tags = local.tags
}

# EKS Cluster Module
module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "~> 19.0"

  cluster_name    = local.cluster_name
  cluster_version = var.kubernetes_version

  cluster_endpoint_public_access = var.cluster_endpoint_public_access

  # Cluster encryption
  cluster_encryption_config = {
    provider_key_arn = aws_kms_key.eks.arn
    resources        = ["secrets"]
  }

  vpc_id     = module.vpc.vpc_id
  subnet_ids = module.vpc.private_subnets

  # IRSA (IAM Roles for Service Accounts)
  enable_irsa = true

  # Cluster addons
  cluster_addons = {
    coredns = {
      most_recent = true
    }
    kube-proxy = {
      most_recent = true
    }
    vpc-cni = {
      most_recent = true
    }
    aws-ebs-csi-driver = {
      most_recent = true
      service_account_role_arn = module.ebs_csi_irsa.iam_role_arn
    }
    aws-efs-csi-driver = {
      most_recent = true
      service_account_role_arn = module.efs_csi_irsa.iam_role_arn
    }
  }

  # Managed node groups
  eks_managed_node_groups = {
    # System node group (controller, monitoring)
    system = {
      name = "${local.cluster_name}-system"

      instance_types = [var.system_instance_type]
      capacity_type  = "ON_DEMAND"

      min_size     = var.system_min_size
      max_size     = var.system_max_size
      desired_size = var.system_desired_size

      labels = {
        role = "system"
        "streamspace.io/node-type" = "system"
      }

      taints = [{
        key    = "streamspace.io/system"
        value  = "true"
        effect = "NO_SCHEDULE"
      }]

      tags = merge(local.tags, {
        NodeGroup = "system"
      })
    }

    # Workload node group (user sessions)
    workload = {
      name = "${local.cluster_name}-workload"

      instance_types = [var.workload_instance_type]
      capacity_type  = var.workload_capacity_type  # SPOT or ON_DEMAND

      min_size     = var.workload_min_size
      max_size     = var.workload_max_size
      desired_size = var.workload_desired_size

      labels = {
        role = "workload"
        "streamspace.io/node-type" = "workload"
      }

      tags = merge(local.tags, {
        NodeGroup = "workload"
      })
    }
  }

  # GPU node group (optional)
  dynamic "eks_managed_node_groups" {
    for_each = var.enable_gpu_nodes ? [1] : []
    content {
      gpu = {
        name = "${local.cluster_name}-gpu"

        instance_types = [var.gpu_instance_type]
        capacity_type  = "ON_DEMAND"

        min_size     = var.gpu_min_size
        max_size     = var.gpu_max_size
        desired_size = var.gpu_desired_size

        ami_type = "AL2_x86_64_GPU"

        labels = {
          role = "gpu"
          "streamspace.io/node-type" = "gpu"
          "nvidia.com/gpu" = "true"
        }

        taints = [{
          key    = "nvidia.com/gpu"
          value  = "true"
          effect = "NO_SCHEDULE"
        }]

        tags = merge(local.tags, {
          NodeGroup = "gpu"
        })
      }
    }
  }

  # Cluster security group rules
  cluster_security_group_additional_rules = {
    ingress_nodes_ephemeral_ports_tcp = {
      description                = "Nodes on ephemeral ports"
      protocol                   = "tcp"
      from_port                  = 1025
      to_port                    = 65535
      type                       = "ingress"
      source_node_security_group = true
    }
  }

  # Node security group rules
  node_security_group_additional_rules = {
    ingress_self_all = {
      description = "Node to node all ports/protocols"
      protocol    = "-1"
      from_port   = 0
      to_port     = 0
      type        = "ingress"
      self        = true
    }
    egress_all = {
      description = "Node all egress"
      protocol    = "-1"
      from_port   = 0
      to_port     = 0
      type        = "egress"
      cidr_blocks = ["0.0.0.0/0"]
    }
  }

  tags = local.tags
}

# KMS key for EKS encryption
resource "aws_kms_key" "eks" {
  description             = "EKS Secret Encryption Key for ${local.cluster_name}"
  deletion_window_in_days = 7
  enable_key_rotation     = true

  tags = local.tags
}

resource "aws_kms_alias" "eks" {
  name          = "alias/${local.cluster_name}-eks"
  target_key_id = aws_kms_key.eks.key_id
}

# EBS CSI Driver IAM Role
module "ebs_csi_irsa" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-role-for-service-accounts-eks"
  version = "~> 5.0"

  role_name = "${local.cluster_name}-ebs-csi"

  attach_ebs_csi_policy = true

  oidc_providers = {
    main = {
      provider_arn               = module.eks.oidc_provider_arn
      namespace_service_accounts = ["kube-system:ebs-csi-controller-sa"]
    }
  }

  tags = local.tags
}

# EFS CSI Driver IAM Role
module "efs_csi_irsa" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-role-for-service-accounts-eks"
  version = "~> 5.0"

  role_name = "${local.cluster_name}-efs-csi"

  attach_efs_csi_policy = true

  oidc_providers = {
    main = {
      provider_arn               = module.eks.oidc_provider_arn
      namespace_service_accounts = ["kube-system:efs-csi-controller-sa"]
    }
  }

  tags = local.tags
}

# EFS for persistent storage
resource "aws_efs_file_system" "streamspace" {
  creation_token = "${local.cluster_name}-efs"
  encrypted      = true
  kms_key_id     = aws_kms_key.efs.arn

  performance_mode = "generalPurpose"
  throughput_mode  = "bursting"

  lifecycle_policy {
    transition_to_ia = "AFTER_30_DAYS"
  }

  tags = merge(local.tags, {
    Name = "${local.cluster_name}-efs"
  })
}

resource "aws_kms_key" "efs" {
  description             = "EFS Encryption Key for ${local.cluster_name}"
  deletion_window_in_days = 7
  enable_key_rotation     = true

  tags = local.tags
}

resource "aws_efs_mount_target" "streamspace" {
  count = length(module.vpc.private_subnets)

  file_system_id  = aws_efs_file_system.streamspace.id
  subnet_id       = module.vpc.private_subnets[count.index]
  security_groups = [aws_security_group.efs.id]
}

resource "aws_security_group" "efs" {
  name        = "${local.cluster_name}-efs-sg"
  description = "Security group for EFS mount targets"
  vpc_id      = module.vpc.vpc_id

  ingress {
    description = "NFS from EKS nodes"
    from_port   = 2049
    to_port     = 2049
    protocol    = "tcp"
    cidr_blocks = module.vpc.private_subnets_cidr_blocks
  }

  egress {
    description = "Allow all outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(local.tags, {
    Name = "${local.cluster_name}-efs-sg"
  })
}

# RDS PostgreSQL (optional - for API database)
module "db" {
  source  = "terraform-aws-modules/rds/aws"
  version = "~> 6.0"

  count = var.enable_rds ? 1 : 0

  identifier = "${local.cluster_name}-db"

  engine               = "postgres"
  engine_version       = "15.4"
  family               = "postgres15"
  major_engine_version = "15"
  instance_class       = var.db_instance_class

  allocated_storage     = var.db_allocated_storage
  max_allocated_storage = var.db_max_allocated_storage

  db_name  = "streamspace"
  username = "streamspace"
  port     = 5432

  multi_az               = var.environment != "dev"
  db_subnet_group_name   = module.vpc.database_subnet_group_name
  vpc_security_group_ids = [aws_security_group.rds[0].id]

  maintenance_window              = "Mon:00:00-Mon:03:00"
  backup_window                   = "03:00-06:00"
  enabled_cloudwatch_logs_exports = ["postgresql", "upgrade"]
  create_cloudwatch_log_group     = true

  backup_retention_period = var.environment == "prod" ? 30 : 7
  skip_final_snapshot     = var.environment != "prod"
  deletion_protection     = var.environment == "prod"

  performance_insights_enabled = true
  performance_insights_retention_period = 7

  storage_encrypted = true
  kms_key_id        = aws_kms_key.rds[0].arn

  tags = local.tags
}

resource "aws_security_group" "rds" {
  count = var.enable_rds ? 1 : 0

  name        = "${local.cluster_name}-rds-sg"
  description = "Security group for RDS PostgreSQL"
  vpc_id      = module.vpc.vpc_id

  ingress {
    description = "PostgreSQL from EKS nodes"
    from_port   = 5432
    to_port     = 5432
    protocol    = "tcp"
    cidr_blocks = module.vpc.private_subnets_cidr_blocks
  }

  tags = merge(local.tags, {
    Name = "${local.cluster_name}-rds-sg"
  })
}

resource "aws_kms_key" "rds" {
  count = var.enable_rds ? 1 : 0

  description             = "RDS Encryption Key for ${local.cluster_name}"
  deletion_window_in_days = 7
  enable_key_rotation     = true

  tags = local.tags
}

# Cluster Autoscaler IAM Role
module "cluster_autoscaler_irsa" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-role-for-service-accounts-eks"
  version = "~> 5.0"

  role_name = "${local.cluster_name}-cluster-autoscaler"

  attach_cluster_autoscaler_policy = true
  cluster_autoscaler_cluster_names = [module.eks.cluster_name]

  oidc_providers = {
    main = {
      provider_arn               = module.eks.oidc_provider_arn
      namespace_service_accounts = ["kube-system:cluster-autoscaler"]
    }
  }

  tags = local.tags
}

# Load Balancer Controller IAM Role
module "load_balancer_controller_irsa" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-role-for-service-accounts-eks"
  version = "~> 5.0"

  role_name = "${local.cluster_name}-aws-load-balancer-controller"

  attach_load_balancer_controller_policy = true

  oidc_providers = {
    main = {
      provider_arn               = module.eks.oidc_provider_arn
      namespace_service_accounts = ["kube-system:aws-load-balancer-controller"]
    }
  }

  tags = local.tags
}
