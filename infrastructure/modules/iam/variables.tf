variable "project_name" {
  description = "Project name used for resource naming"
  type        = string
}

variable "environment" {
  description = "Deployment environment"
  type        = string
}

variable "s3_bucket_arn" {
  description = "ARN of the S3 bucket"
  type        = string
}

variable "dynamodb_users_table_arn" {
  description = "ARN of the DynamoDB Users table"
  type        = string
}

variable "dynamodb_projects_table_arn" {
  description = "ARN of the DynamoDB Projects table"
  type        = string
}

variable "dynamodb_sessions_table_arn" {
  description = "ARN of the DynamoDB Sessions table"
  type        = string
}

variable "step_functions_arn" {
  description = "ARN of the Step Functions state machine"
  type        = string
}

variable "lambda_function_arns" {
  description = "List of Lambda function ARNs for Step Functions to invoke"
  type        = list(string)
}

variable "tags" {
  description = "Common tags applied to all resources"
  type        = map(string)
  default     = {}
}
