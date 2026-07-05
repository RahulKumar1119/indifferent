resource "aws_dynamodb_table" "users" {
  name         = "${var.project_name}-users-${var.environment}"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "PK"
  range_key    = "SK"

  attribute {
    name = "PK"
    type = "S"
  }

  attribute {
    name = "SK"
    type = "S"
  }

  tags = merge(var.tags, {
    Name = "${var.project_name}-users-${var.environment}"
  })
}

resource "aws_dynamodb_table" "projects" {
  name         = "${var.project_name}-projects-${var.environment}"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "PK"
  range_key    = "SK"

  attribute {
    name = "PK"
    type = "S"
  }

  attribute {
    name = "SK"
    type = "S"
  }

  tags = merge(var.tags, {
    Name = "${var.project_name}-projects-${var.environment}"
  })
}

resource "aws_dynamodb_table" "sessions" {
  name         = "${var.project_name}-sessions-${var.environment}"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "PK"
  range_key    = "SK"

  attribute {
    name = "PK"
    type = "S"
  }

  attribute {
    name = "SK"
    type = "S"
  }

  ttl {
    attribute_name = "expiresAt"
    enabled        = true
  }

  tags = merge(var.tags, {
    Name = "${var.project_name}-sessions-${var.environment}"
  })
}
