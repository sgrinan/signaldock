variable "aws_region" {
  description = "AWS region used by SignalDock"
  type        = string
  default     = "eu-west-1"
}

variable "project_name" {
  description = "Project name used for AWS resource naming and tagging"
  type        = string
  default     = "signaldock"
}

variable "environment" {
  description = "Deployment environment"
  type        = string
  default     = "lab"
}