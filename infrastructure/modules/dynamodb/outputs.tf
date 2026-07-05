output "users_table_name" {
  description = "Name of the Users DynamoDB table"
  value       = aws_dynamodb_table.users.name
}

output "users_table_arn" {
  description = "ARN of the Users DynamoDB table"
  value       = aws_dynamodb_table.users.arn
}

output "projects_table_name" {
  description = "Name of the Projects DynamoDB table"
  value       = aws_dynamodb_table.projects.name
}

output "projects_table_arn" {
  description = "ARN of the Projects DynamoDB table"
  value       = aws_dynamodb_table.projects.arn
}

output "sessions_table_name" {
  description = "Name of the Sessions DynamoDB table"
  value       = aws_dynamodb_table.sessions.name
}

output "sessions_table_arn" {
  description = "ARN of the Sessions DynamoDB table"
  value       = aws_dynamodb_table.sessions.arn
}
