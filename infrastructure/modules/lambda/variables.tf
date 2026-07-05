variable "project_name" {
  description = "Project name used for resource naming"
  type        = string
}

variable "environment" {
  description = "Deployment environment"
  type        = string
}

variable "api_lambda_role_arn" {
  description = "IAM role ARN for the API Lambda"
  type        = string
}

variable "parser_lambda_role_arn" {
  description = "IAM role ARN for the Parser Lambda"
  type        = string
}

variable "slidegen_lambda_role_arn" {
  description = "IAM role ARN for the Slide Generator Lambda"
  type        = string
}

variable "narrator_lambda_role_arn" {
  description = "IAM role ARN for the Narrator Lambda"
  type        = string
}

variable "renderer_lambda_role_arn" {
  description = "IAM role ARN for the Renderer Lambda"
  type        = string
}

variable "dynamodb_users_table_name" {
  description = "DynamoDB Users table name"
  type        = string
}

variable "dynamodb_projects_table_name" {
  description = "DynamoDB Projects table name"
  type        = string
}

variable "dynamodb_sessions_table_name" {
  description = "DynamoDB Sessions table name"
  type        = string
}

variable "s3_bucket_name" {
  description = "S3 bucket name"
  type        = string
}

variable "step_functions_arn" {
  description = "Step Functions state machine ARN"
  type        = string
}

variable "google_client_id" {
  description = "Google OAuth Client ID"
  type        = string
  sensitive   = true
}

variable "google_client_secret" {
  description = "Google OAuth Client Secret"
  type        = string
  sensitive   = true
}

variable "jwt_secret" {
  description = "JWT signing secret"
  type        = string
  sensitive   = true
}

variable "frontend_domain" {
  description = "Frontend domain for CORS"
  type        = string
}

variable "api_lambda_zip_path" {
  description = "Path to the API Lambda deployment package"
  type        = string
  default     = "../backend/build/api.zip"
}

variable "parser_lambda_zip_path" {
  description = "Path to the Parser Lambda deployment package"
  type        = string
  default     = "../backend/build/parser.zip"
}

variable "slidegen_lambda_zip_path" {
  description = "Path to the Slide Generator Lambda deployment package"
  type        = string
  default     = "../backend/build/slidegen.zip"
}

variable "narrator_lambda_zip_path" {
  description = "Path to the Narrator Lambda deployment package"
  type        = string
  default     = "../backend/build/narrator.zip"
}

variable "renderer_image_uri" {
  description = "ECR image URI for the Renderer container Lambda"
  type        = string
}

variable "tags" {
  description = "Common tags applied to all resources"
  type        = map(string)
  default     = {}
}
