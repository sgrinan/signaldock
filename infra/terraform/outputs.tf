output "vpc_id" {
  description = "SignalDock VPC ID"
  value       = aws_vpc.main.id
}

output "public_subnet_ids" {
  description = "Public subnet IDs used by EKS"
  value       = aws_subnet.public[*].id
}

output "availability_zones" {
  description = "Availability Zones used by the public subnets"
  value       = aws_subnet.public[*].availability_zone
}

output "eks_cluster_name" {
  description = "EKS cluster name"
  value       = aws_eks_cluster.main.name
}

output "eks_cluster_endpoint" {
  description = "EKS API server endpoint"
  value       = aws_eks_cluster.main.endpoint
}

output "eks_node_group_name" {
  description = "EKS managed node group name"
  value       = aws_eks_node_group.main.node_group_name
}