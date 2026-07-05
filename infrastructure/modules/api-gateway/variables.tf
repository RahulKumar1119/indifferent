variable "project_name" {
  description = "Project name used for resource naming"
  type        = string
}

variable "environment" {
  description = "Deployment environment"
  type        = string
}

variable "api_lambda_invoke_arn" {
  description = "API Lambda invoke ARN for integration"
  type        = string
}

variable "api_lambda_function_name" {
  description = "API Lambda function name for permission"
  type        = string
}

variable "authorizer_invoke_arn" {
  description = "Authorizer Lambda invoke ARN"
  type        = string
}

variable "authorizer_role_arn" {
  description = "IAM role ARN for the API Gateway to invoke the authorizer"
  type        = string
}

variable "frontend_domain" {
  description = "Frontend domain for CORS"
  type        = string
}

variable "tags" {
  description = "Common tags applied to all resources"
  type        = map(string)
  default     = {}
}
