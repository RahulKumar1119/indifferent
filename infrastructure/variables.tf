variable "project_name" {
  description = "Project name used for resource naming"
  type        = string
  default     = "txt-to-video"
}

variable "environment" {
  description = "Deployment environment (staging or production)"
  type        = string
  validation {
    condition     = contains(["staging", "production"], var.environment)
    error_message = "Environment must be either 'staging' or 'production'."
  }
}

variable "aws_region" {
  description = "AWS region for resource deployment"
  type        = string
  default     = "us-east-1"
}

variable "google_client_id" {
  description = "Google OAuth 2.0 Client ID"
  type        = string
  sensitive   = true
}

variable "google_client_secret" {
  description = "Google OAuth 2.0 Client Secret"
  type        = string
  sensitive   = true
}

variable "jwt_secret" {
  description = "Secret key for JWT token signing"
  type        = string
  sensitive   = true
}

variable "frontend_domain" {
  description = "Frontend domain for CORS configuration"
  type        = string
}

variable "tags" {
  description = "Common tags applied to all resources"
  type        = map(string)
  default     = {}
}
