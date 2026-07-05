variable "rest_api_id" {
  description = "REST API ID"
  type        = string
}

variable "resource_id" {
  description = "API Gateway resource ID"
  type        = string
}

variable "allowed_origins" {
  description = "Allowed CORS origins"
  type        = string
}

variable "allowed_methods" {
  description = "Allowed HTTP methods (comma-separated)"
  type        = string
}
