terraform {
  required_version = ">= 1.5.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }

  backend "s3" {
    bucket         = "txt-to-video-terraform-state"
    key            = "production/terraform.tfstate"
    region         = "us-east-1"
    dynamodb_table = "txt-to-video-terraform-locks"
    encrypt        = true
  }
}

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      Project     = "txt-to-video"
      Environment = "production"
      ManagedBy   = "terraform"
    }
  }
}

locals {
  environment  = "production"
  project_name = "txt-to-video"

  tags = {
    Project     = "txt-to-video"
    Environment = "production"
    ManagedBy   = "terraform"
  }
}

# ----- DynamoDB -----

module "dynamodb" {
  source = "../../modules/dynamodb"

  project_name = local.project_name
  environment  = local.environment
  tags         = local.tags
}

# ----- S3 -----

module "s3" {
  source = "../../modules/s3"

  project_name = local.project_name
  environment  = local.environment
  tags         = local.tags
}

# ----- Lambda -----

module "lambda" {
  source = "../../modules/lambda"

  project_name = local.project_name
  environment  = local.environment

  api_lambda_role_arn      = module.iam.api_lambda_role_arn
  parser_lambda_role_arn   = module.iam.parser_lambda_role_arn
  slidegen_lambda_role_arn = module.iam.slidegen_lambda_role_arn
  narrator_lambda_role_arn = module.iam.narrator_lambda_role_arn
  renderer_lambda_role_arn = module.iam.renderer_lambda_role_arn

  dynamodb_users_table_name    = module.dynamodb.users_table_name
  dynamodb_projects_table_name = module.dynamodb.projects_table_name
  dynamodb_sessions_table_name = module.dynamodb.sessions_table_name
  s3_bucket_name               = module.s3.bucket_name
  step_functions_arn           = module.step_functions.state_machine_arn

  google_client_id     = var.google_client_id
  google_client_secret = var.google_client_secret
  jwt_secret           = var.jwt_secret
  frontend_domain      = var.frontend_domain
  renderer_image_uri   = var.renderer_image_uri

  tags = local.tags
}

# ----- Step Functions -----

module "step_functions" {
  source = "../../modules/step-functions"

  project_name                 = local.project_name
  environment                  = local.environment
  step_functions_role_arn      = module.iam.step_functions_role_arn
  parser_lambda_arn            = module.lambda.parser_function_arn
  slidegen_lambda_arn          = module.lambda.slidegen_function_arn
  narrator_lambda_arn          = module.lambda.narrator_function_arn
  renderer_lambda_arn          = module.lambda.renderer_function_arn
  dynamodb_projects_table_name = module.dynamodb.projects_table_name
  tags                         = local.tags
}

# ----- IAM -----

module "iam" {
  source = "../../modules/iam"

  project_name                = local.project_name
  environment                 = local.environment
  s3_bucket_arn               = module.s3.bucket_arn
  dynamodb_users_table_arn    = module.dynamodb.users_table_arn
  dynamodb_projects_table_arn = module.dynamodb.projects_table_arn
  dynamodb_sessions_table_arn = module.dynamodb.sessions_table_arn
  step_functions_arn          = module.step_functions.state_machine_arn
  lambda_function_arns        = module.lambda.all_function_arns
  tags                        = local.tags
}

# ----- API Gateway -----

module "api_gateway" {
  source = "../../modules/api-gateway"

  project_name             = local.project_name
  environment              = local.environment
  api_lambda_invoke_arn    = module.lambda.api_invoke_arn
  api_lambda_function_name = module.lambda.api_function_name
  authorizer_invoke_arn    = module.lambda.api_invoke_arn
  authorizer_role_arn      = module.iam.authorizer_lambda_role_arn
  frontend_domain          = var.frontend_domain
  tags                     = local.tags
}

# ----- Variables -----

variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
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
  default     = "https://txt-to-video.com"
}

variable "renderer_image_uri" {
  description = "ECR image URI for the Renderer container"
  type        = string
}

# ----- Outputs -----

output "api_gateway_url" {
  description = "Production API Gateway URL"
  value       = module.api_gateway.invoke_url
}

output "s3_bucket_name" {
  description = "Production S3 bucket name"
  value       = module.s3.bucket_name
}
