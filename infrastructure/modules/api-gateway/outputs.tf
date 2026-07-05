output "rest_api_id" {
  description = "REST API ID"
  value       = aws_api_gateway_rest_api.main.id
}

output "invoke_url" {
  description = "API Gateway invoke URL"
  value       = aws_api_gateway_stage.main.invoke_url
}

output "execution_arn" {
  description = "API Gateway execution ARN"
  value       = aws_api_gateway_rest_api.main.execution_arn
}

output "stage_name" {
  description = "API Gateway stage name"
  value       = aws_api_gateway_stage.main.stage_name
}
