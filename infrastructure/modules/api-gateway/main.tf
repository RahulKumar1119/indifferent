# API Gateway REST API for TXT-to-Video SaaS

resource "aws_api_gateway_rest_api" "main" {
  name        = "${var.project_name}-api-${var.environment}"
  description = "TXT-to-Video SaaS REST API"

  endpoint_configuration {
    types = ["REGIONAL"]
  }

  tags = merge(var.tags, {
    Name = "${var.project_name}-api-${var.environment}"
  })
}

# ----- Custom Authorizer -----

resource "aws_api_gateway_authorizer" "jwt" {
  name                             = "${var.project_name}-jwt-authorizer-${var.environment}"
  rest_api_id                      = aws_api_gateway_rest_api.main.id
  authorizer_uri                   = var.authorizer_invoke_arn
  authorizer_credentials           = var.authorizer_role_arn
  type                             = "TOKEN"
  identity_source                  = "method.request.header.Authorization"
  authorizer_result_ttl_in_seconds = 300
}

# ----- /auth Resource -----

resource "aws_api_gateway_resource" "auth" {
  rest_api_id = aws_api_gateway_rest_api.main.id
  parent_id   = aws_api_gateway_rest_api.main.root_resource_id
  path_part   = "auth"
}

resource "aws_api_gateway_resource" "auth_google" {
  rest_api_id = aws_api_gateway_rest_api.main.id
  parent_id   = aws_api_gateway_resource.auth.id
  path_part   = "google"
}

resource "aws_api_gateway_resource" "auth_google_callback" {
  rest_api_id = aws_api_gateway_rest_api.main.id
  parent_id   = aws_api_gateway_resource.auth_google.id
  path_part   = "callback"
}

resource "aws_api_gateway_resource" "auth_refresh" {
  rest_api_id = aws_api_gateway_rest_api.main.id
  parent_id   = aws_api_gateway_resource.auth.id
  path_part   = "refresh"
}

resource "aws_api_gateway_resource" "auth_logout" {
  rest_api_id = aws_api_gateway_rest_api.main.id
  parent_id   = aws_api_gateway_resource.auth.id
  path_part   = "logout"
}

# ----- /projects Resource -----

resource "aws_api_gateway_resource" "projects" {
  rest_api_id = aws_api_gateway_rest_api.main.id
  parent_id   = aws_api_gateway_rest_api.main.root_resource_id
  path_part   = "projects"
}

resource "aws_api_gateway_resource" "project_id" {
  rest_api_id = aws_api_gateway_rest_api.main.id
  parent_id   = aws_api_gateway_resource.projects.id
  path_part   = "{id}"
}

resource "aws_api_gateway_resource" "project_upload" {
  rest_api_id = aws_api_gateway_rest_api.main.id
  parent_id   = aws_api_gateway_resource.project_id.id
  path_part   = "upload"
}

resource "aws_api_gateway_resource" "project_status" {
  rest_api_id = aws_api_gateway_rest_api.main.id
  parent_id   = aws_api_gateway_resource.project_id.id
  path_part   = "status"
}

resource "aws_api_gateway_resource" "project_download" {
  rest_api_id = aws_api_gateway_rest_api.main.id
  parent_id   = aws_api_gateway_resource.project_id.id
  path_part   = "download"
}

# ----- Auth Methods (No authorization required) -----

resource "aws_api_gateway_method" "auth_google_callback_post" {
  rest_api_id   = aws_api_gateway_rest_api.main.id
  resource_id   = aws_api_gateway_resource.auth_google_callback.id
  http_method   = "POST"
  authorization = "NONE"
}

resource "aws_api_gateway_method" "auth_refresh_post" {
  rest_api_id   = aws_api_gateway_rest_api.main.id
  resource_id   = aws_api_gateway_resource.auth_refresh.id
  http_method   = "POST"
  authorization = "NONE"
}

resource "aws_api_gateway_method" "auth_logout_post" {
  rest_api_id   = aws_api_gateway_rest_api.main.id
  resource_id   = aws_api_gateway_resource.auth_logout.id
  http_method   = "POST"
  authorization = "CUSTOM"
  authorizer_id = aws_api_gateway_authorizer.jwt.id
}

# ----- Projects Methods (Authorization required) -----

resource "aws_api_gateway_method" "projects_get" {
  rest_api_id   = aws_api_gateway_rest_api.main.id
  resource_id   = aws_api_gateway_resource.projects.id
  http_method   = "GET"
  authorization = "CUSTOM"
  authorizer_id = aws_api_gateway_authorizer.jwt.id
}

resource "aws_api_gateway_method" "projects_post" {
  rest_api_id   = aws_api_gateway_rest_api.main.id
  resource_id   = aws_api_gateway_resource.projects.id
  http_method   = "POST"
  authorization = "CUSTOM"
  authorizer_id = aws_api_gateway_authorizer.jwt.id
}

resource "aws_api_gateway_method" "project_get" {
  rest_api_id   = aws_api_gateway_rest_api.main.id
  resource_id   = aws_api_gateway_resource.project_id.id
  http_method   = "GET"
  authorization = "CUSTOM"
  authorizer_id = aws_api_gateway_authorizer.jwt.id
}

resource "aws_api_gateway_method" "project_delete" {
  rest_api_id   = aws_api_gateway_rest_api.main.id
  resource_id   = aws_api_gateway_resource.project_id.id
  http_method   = "DELETE"
  authorization = "CUSTOM"
  authorizer_id = aws_api_gateway_authorizer.jwt.id
}

resource "aws_api_gateway_method" "project_upload_post" {
  rest_api_id   = aws_api_gateway_rest_api.main.id
  resource_id   = aws_api_gateway_resource.project_upload.id
  http_method   = "POST"
  authorization = "CUSTOM"
  authorizer_id = aws_api_gateway_authorizer.jwt.id
}

resource "aws_api_gateway_method" "project_status_get" {
  rest_api_id   = aws_api_gateway_rest_api.main.id
  resource_id   = aws_api_gateway_resource.project_status.id
  http_method   = "GET"
  authorization = "CUSTOM"
  authorizer_id = aws_api_gateway_authorizer.jwt.id
}

resource "aws_api_gateway_method" "project_download_get" {
  rest_api_id   = aws_api_gateway_rest_api.main.id
  resource_id   = aws_api_gateway_resource.project_download.id
  http_method   = "GET"
  authorization = "CUSTOM"
  authorizer_id = aws_api_gateway_authorizer.jwt.id
}

# ----- Lambda Integrations -----

resource "aws_api_gateway_integration" "auth_google_callback" {
  rest_api_id             = aws_api_gateway_rest_api.main.id
  resource_id             = aws_api_gateway_resource.auth_google_callback.id
  http_method             = aws_api_gateway_method.auth_google_callback_post.http_method
  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = var.api_lambda_invoke_arn
}

resource "aws_api_gateway_integration" "auth_refresh" {
  rest_api_id             = aws_api_gateway_rest_api.main.id
  resource_id             = aws_api_gateway_resource.auth_refresh.id
  http_method             = aws_api_gateway_method.auth_refresh_post.http_method
  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = var.api_lambda_invoke_arn
}

resource "aws_api_gateway_integration" "auth_logout" {
  rest_api_id             = aws_api_gateway_rest_api.main.id
  resource_id             = aws_api_gateway_resource.auth_logout.id
  http_method             = aws_api_gateway_method.auth_logout_post.http_method
  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = var.api_lambda_invoke_arn
}

resource "aws_api_gateway_integration" "projects_get" {
  rest_api_id             = aws_api_gateway_rest_api.main.id
  resource_id             = aws_api_gateway_resource.projects.id
  http_method             = aws_api_gateway_method.projects_get.http_method
  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = var.api_lambda_invoke_arn
}

resource "aws_api_gateway_integration" "projects_post" {
  rest_api_id             = aws_api_gateway_rest_api.main.id
  resource_id             = aws_api_gateway_resource.projects.id
  http_method             = aws_api_gateway_method.projects_post.http_method
  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = var.api_lambda_invoke_arn
}

resource "aws_api_gateway_integration" "project_get" {
  rest_api_id             = aws_api_gateway_rest_api.main.id
  resource_id             = aws_api_gateway_resource.project_id.id
  http_method             = aws_api_gateway_method.project_get.http_method
  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = var.api_lambda_invoke_arn
}

resource "aws_api_gateway_integration" "project_delete" {
  rest_api_id             = aws_api_gateway_rest_api.main.id
  resource_id             = aws_api_gateway_resource.project_id.id
  http_method             = aws_api_gateway_method.project_delete.http_method
  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = var.api_lambda_invoke_arn
}

resource "aws_api_gateway_integration" "project_upload" {
  rest_api_id             = aws_api_gateway_rest_api.main.id
  resource_id             = aws_api_gateway_resource.project_upload.id
  http_method             = aws_api_gateway_method.project_upload_post.http_method
  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = var.api_lambda_invoke_arn
}

resource "aws_api_gateway_integration" "project_status" {
  rest_api_id             = aws_api_gateway_rest_api.main.id
  resource_id             = aws_api_gateway_resource.project_status.id
  http_method             = aws_api_gateway_method.project_status_get.http_method
  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = var.api_lambda_invoke_arn
}

resource "aws_api_gateway_integration" "project_download" {
  rest_api_id             = aws_api_gateway_rest_api.main.id
  resource_id             = aws_api_gateway_resource.project_download.id
  http_method             = aws_api_gateway_method.project_download_get.http_method
  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = var.api_lambda_invoke_arn
}

# ----- CORS Configuration (OPTIONS methods) -----

module "cors_auth_google_callback" {
  source          = "./cors"
  rest_api_id     = aws_api_gateway_rest_api.main.id
  resource_id     = aws_api_gateway_resource.auth_google_callback.id
  allowed_origins = var.frontend_domain
  allowed_methods = "POST,OPTIONS"
}

module "cors_auth_refresh" {
  source          = "./cors"
  rest_api_id     = aws_api_gateway_rest_api.main.id
  resource_id     = aws_api_gateway_resource.auth_refresh.id
  allowed_origins = var.frontend_domain
  allowed_methods = "POST,OPTIONS"
}

module "cors_auth_logout" {
  source          = "./cors"
  rest_api_id     = aws_api_gateway_rest_api.main.id
  resource_id     = aws_api_gateway_resource.auth_logout.id
  allowed_origins = var.frontend_domain
  allowed_methods = "POST,OPTIONS"
}

module "cors_projects" {
  source          = "./cors"
  rest_api_id     = aws_api_gateway_rest_api.main.id
  resource_id     = aws_api_gateway_resource.projects.id
  allowed_origins = var.frontend_domain
  allowed_methods = "GET,POST,OPTIONS"
}

module "cors_project_id" {
  source          = "./cors"
  rest_api_id     = aws_api_gateway_rest_api.main.id
  resource_id     = aws_api_gateway_resource.project_id.id
  allowed_origins = var.frontend_domain
  allowed_methods = "GET,DELETE,OPTIONS"
}

module "cors_project_upload" {
  source          = "./cors"
  rest_api_id     = aws_api_gateway_rest_api.main.id
  resource_id     = aws_api_gateway_resource.project_upload.id
  allowed_origins = var.frontend_domain
  allowed_methods = "POST,OPTIONS"
}

module "cors_project_status" {
  source          = "./cors"
  rest_api_id     = aws_api_gateway_rest_api.main.id
  resource_id     = aws_api_gateway_resource.project_status.id
  allowed_origins = var.frontend_domain
  allowed_methods = "GET,OPTIONS"
}

module "cors_project_download" {
  source          = "./cors"
  rest_api_id     = aws_api_gateway_rest_api.main.id
  resource_id     = aws_api_gateway_resource.project_download.id
  allowed_origins = var.frontend_domain
  allowed_methods = "GET,OPTIONS"
}

# ----- Deployment -----

resource "aws_api_gateway_deployment" "main" {
  rest_api_id = aws_api_gateway_rest_api.main.id

  depends_on = [
    aws_api_gateway_integration.auth_google_callback,
    aws_api_gateway_integration.auth_refresh,
    aws_api_gateway_integration.auth_logout,
    aws_api_gateway_integration.projects_get,
    aws_api_gateway_integration.projects_post,
    aws_api_gateway_integration.project_get,
    aws_api_gateway_integration.project_delete,
    aws_api_gateway_integration.project_upload,
    aws_api_gateway_integration.project_status,
    aws_api_gateway_integration.project_download,
  ]

  triggers = {
    redeployment = sha1(jsonencode([
      aws_api_gateway_resource.auth_google_callback,
      aws_api_gateway_resource.auth_refresh,
      aws_api_gateway_resource.auth_logout,
      aws_api_gateway_resource.projects,
      aws_api_gateway_resource.project_id,
      aws_api_gateway_resource.project_upload,
      aws_api_gateway_resource.project_status,
      aws_api_gateway_resource.project_download,
    ]))
  }

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_api_gateway_stage" "main" {
  deployment_id = aws_api_gateway_deployment.main.id
  rest_api_id   = aws_api_gateway_rest_api.main.id
  stage_name    = var.environment
}

# ----- Lambda Permission for API Gateway -----

resource "aws_lambda_permission" "api_gateway" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = var.api_lambda_function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_api_gateway_rest_api.main.execution_arn}/*/*"
}
