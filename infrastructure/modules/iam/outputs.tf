output "api_lambda_role_arn" {
  description = "IAM role ARN for the API Lambda"
  value       = aws_iam_role.api_lambda.arn
}

output "parser_lambda_role_arn" {
  description = "IAM role ARN for the Parser Lambda"
  value       = aws_iam_role.parser_lambda.arn
}

output "slidegen_lambda_role_arn" {
  description = "IAM role ARN for the Slide Generator Lambda"
  value       = aws_iam_role.slidegen_lambda.arn
}

output "narrator_lambda_role_arn" {
  description = "IAM role ARN for the Narrator Lambda"
  value       = aws_iam_role.narrator_lambda.arn
}

output "renderer_lambda_role_arn" {
  description = "IAM role ARN for the Renderer Lambda"
  value       = aws_iam_role.renderer_lambda.arn
}

output "step_functions_role_arn" {
  description = "IAM role ARN for Step Functions"
  value       = aws_iam_role.step_functions.arn
}

output "authorizer_lambda_role_arn" {
  description = "IAM role ARN for the API Gateway Authorizer Lambda"
  value       = aws_iam_role.authorizer_lambda.arn
}
