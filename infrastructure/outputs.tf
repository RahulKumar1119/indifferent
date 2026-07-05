output "api_gateway_url" {
  description = "API Gateway invoke URL"
  value       = module.api_gateway.invoke_url
}

output "s3_bucket_name" {
  description = "S3 bucket name for file storage"
  value       = module.s3.bucket_name
}

output "dynamodb_users_table" {
  description = "DynamoDB Users table name"
  value       = module.dynamodb.users_table_name
}

output "dynamodb_projects_table" {
  description = "DynamoDB Projects table name"
  value       = module.dynamodb.projects_table_name
}

output "dynamodb_sessions_table" {
  description = "DynamoDB Sessions table name"
  value       = module.dynamodb.sessions_table_name
}

output "step_functions_arn" {
  description = "Step Functions state machine ARN"
  value       = module.step_functions.state_machine_arn
}

output "lambda_api_function_name" {
  description = "API Lambda function name"
  value       = module.lambda.api_function_name
}

output "lambda_parser_function_name" {
  description = "Parser Lambda function name"
  value       = module.lambda.parser_function_name
}

output "lambda_slidegen_function_name" {
  description = "Slide Generator Lambda function name"
  value       = module.lambda.slidegen_function_name
}

output "lambda_narrator_function_name" {
  description = "Narrator Lambda function name"
  value       = module.lambda.narrator_function_name
}

output "lambda_renderer_function_name" {
  description = "Renderer Lambda function name"
  value       = module.lambda.renderer_function_name
}
