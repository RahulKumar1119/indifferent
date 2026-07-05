output "api_function_name" {
  description = "API Lambda function name"
  value       = aws_lambda_function.api.function_name
}

output "api_function_arn" {
  description = "API Lambda function ARN"
  value       = aws_lambda_function.api.arn
}

output "api_invoke_arn" {
  description = "API Lambda invoke ARN (for API Gateway integration)"
  value       = aws_lambda_function.api.invoke_arn
}

output "parser_function_name" {
  description = "Parser Lambda function name"
  value       = aws_lambda_function.parser.function_name
}

output "parser_function_arn" {
  description = "Parser Lambda function ARN"
  value       = aws_lambda_function.parser.arn
}

output "slidegen_function_name" {
  description = "Slide Generator Lambda function name"
  value       = aws_lambda_function.slidegen.function_name
}

output "slidegen_function_arn" {
  description = "Slide Generator Lambda function ARN"
  value       = aws_lambda_function.slidegen.arn
}

output "narrator_function_name" {
  description = "Narrator Lambda function name"
  value       = aws_lambda_function.narrator.function_name
}

output "narrator_function_arn" {
  description = "Narrator Lambda function ARN"
  value       = aws_lambda_function.narrator.arn
}

output "renderer_function_name" {
  description = "Renderer Lambda function name"
  value       = aws_lambda_function.renderer.function_name
}

output "renderer_function_arn" {
  description = "Renderer Lambda function ARN"
  value       = aws_lambda_function.renderer.arn
}

output "all_function_arns" {
  description = "List of all Lambda function ARNs"
  value = [
    aws_lambda_function.parser.arn,
    aws_lambda_function.slidegen.arn,
    aws_lambda_function.narrator.arn,
    aws_lambda_function.renderer.arn
  ]
}
