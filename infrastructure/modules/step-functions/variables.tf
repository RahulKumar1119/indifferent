variable "project_name" {
  description = "Project name used for resource naming"
  type        = string
}

variable "environment" {
  description = "Deployment environment"
  type        = string
}

variable "step_functions_role_arn" {
  description = "IAM role ARN for Step Functions"
  type        = string
}

variable "parser_lambda_arn" {
  description = "ARN of the Parser Lambda function"
  type        = string
}

variable "slidegen_lambda_arn" {
  description = "ARN of the Slide Generator Lambda function"
  type        = string
}

variable "narrator_lambda_arn" {
  description = "ARN of the Narrator Lambda function"
  type        = string
}

variable "renderer_lambda_arn" {
  description = "ARN of the Renderer Lambda function"
  type        = string
}

variable "dynamodb_projects_table_name" {
  description = "DynamoDB Projects table name"
  type        = string
}

variable "tags" {
  description = "Common tags applied to all resources"
  type        = map(string)
  default     = {}
}
