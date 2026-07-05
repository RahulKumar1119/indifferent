# IAM roles and policies for Lambda functions with least-privilege permissions

data "aws_iam_policy_document" "lambda_assume_role" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

# ----- API Lambda Role -----

resource "aws_iam_role" "api_lambda" {
  name               = "${var.project_name}-api-lambda-${var.environment}"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume_role.json

  tags = merge(var.tags, {
    Name = "${var.project_name}-api-lambda-${var.environment}"
  })
}

resource "aws_iam_role_policy" "api_lambda" {
  name   = "${var.project_name}-api-lambda-policy-${var.environment}"
  role   = aws_iam_role.api_lambda.id
  policy = data.aws_iam_policy_document.api_lambda_policy.json
}

data "aws_iam_policy_document" "api_lambda_policy" {
  # CloudWatch Logs
  statement {
    actions = [
      "logs:CreateLogGroup",
      "logs:CreateLogStream",
      "logs:PutLogEvents"
    ]
    resources = ["arn:aws:logs:*:*:*"]
  }

  # DynamoDB access for Users, Projects, and Sessions tables
  statement {
    actions = [
      "dynamodb:GetItem",
      "dynamodb:PutItem",
      "dynamodb:UpdateItem",
      "dynamodb:DeleteItem",
      "dynamodb:Query"
    ]
    resources = [
      var.dynamodb_users_table_arn,
      var.dynamodb_projects_table_arn,
      var.dynamodb_sessions_table_arn
    ]
  }

  # S3 access for uploads and signed URL generation
  statement {
    actions = [
      "s3:PutObject",
      "s3:GetObject",
      "s3:DeleteObject"
    ]
    resources = ["${var.s3_bucket_arn}/*"]
  }

  # Step Functions to start pipeline execution
  statement {
    actions = [
      "states:StartExecution"
    ]
    resources = [var.step_functions_arn]
  }
}

resource "aws_iam_role_policy_attachment" "api_lambda_basic" {
  role       = aws_iam_role.api_lambda.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

# ----- Parser Lambda Role -----

resource "aws_iam_role" "parser_lambda" {
  name               = "${var.project_name}-parser-lambda-${var.environment}"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume_role.json

  tags = merge(var.tags, {
    Name = "${var.project_name}-parser-lambda-${var.environment}"
  })
}

resource "aws_iam_role_policy" "parser_lambda" {
  name   = "${var.project_name}-parser-lambda-policy-${var.environment}"
  role   = aws_iam_role.parser_lambda.id
  policy = data.aws_iam_policy_document.parser_lambda_policy.json
}

data "aws_iam_policy_document" "parser_lambda_policy" {
  # CloudWatch Logs
  statement {
    actions = [
      "logs:CreateLogGroup",
      "logs:CreateLogStream",
      "logs:PutLogEvents"
    ]
    resources = ["arn:aws:logs:*:*:*"]
  }

  # S3: read uploads, write parsed output
  statement {
    actions = [
      "s3:GetObject"
    ]
    resources = ["${var.s3_bucket_arn}/uploads/*"]
  }

  statement {
    actions = [
      "s3:PutObject"
    ]
    resources = ["${var.s3_bucket_arn}/parsed/*"]
  }
}

resource "aws_iam_role_policy_attachment" "parser_lambda_basic" {
  role       = aws_iam_role.parser_lambda.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

# ----- Slide Generator Lambda Role -----

resource "aws_iam_role" "slidegen_lambda" {
  name               = "${var.project_name}-slidegen-lambda-${var.environment}"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume_role.json

  tags = merge(var.tags, {
    Name = "${var.project_name}-slidegen-lambda-${var.environment}"
  })
}

resource "aws_iam_role_policy" "slidegen_lambda" {
  name   = "${var.project_name}-slidegen-lambda-policy-${var.environment}"
  role   = aws_iam_role.slidegen_lambda.id
  policy = data.aws_iam_policy_document.slidegen_lambda_policy.json
}

data "aws_iam_policy_document" "slidegen_lambda_policy" {
  # CloudWatch Logs
  statement {
    actions = [
      "logs:CreateLogGroup",
      "logs:CreateLogStream",
      "logs:PutLogEvents"
    ]
    resources = ["arn:aws:logs:*:*:*"]
  }

  # S3: read parsed JSON, write slide PNGs to temp
  statement {
    actions = [
      "s3:GetObject"
    ]
    resources = ["${var.s3_bucket_arn}/parsed/*"]
  }

  statement {
    actions = [
      "s3:PutObject"
    ]
    resources = ["${var.s3_bucket_arn}/temp/*"]
  }
}

resource "aws_iam_role_policy_attachment" "slidegen_lambda_basic" {
  role       = aws_iam_role.slidegen_lambda.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

# ----- Narrator Lambda Role -----

resource "aws_iam_role" "narrator_lambda" {
  name               = "${var.project_name}-narrator-lambda-${var.environment}"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume_role.json

  tags = merge(var.tags, {
    Name = "${var.project_name}-narrator-lambda-${var.environment}"
  })
}

resource "aws_iam_role_policy" "narrator_lambda" {
  name   = "${var.project_name}-narrator-lambda-policy-${var.environment}"
  role   = aws_iam_role.narrator_lambda.id
  policy = data.aws_iam_policy_document.narrator_lambda_policy.json
}

data "aws_iam_policy_document" "narrator_lambda_policy" {
  # CloudWatch Logs
  statement {
    actions = [
      "logs:CreateLogGroup",
      "logs:CreateLogStream",
      "logs:PutLogEvents"
    ]
    resources = ["arn:aws:logs:*:*:*"]
  }

  # S3: read parsed JSON, write audio to temp
  statement {
    actions = [
      "s3:GetObject"
    ]
    resources = ["${var.s3_bucket_arn}/parsed/*"]
  }

  statement {
    actions = [
      "s3:PutObject"
    ]
    resources = ["${var.s3_bucket_arn}/temp/*"]
  }

  # Amazon Polly: synthesize speech
  statement {
    actions = [
      "polly:SynthesizeSpeech"
    ]
    resources = ["*"]
  }
}

resource "aws_iam_role_policy_attachment" "narrator_lambda_basic" {
  role       = aws_iam_role.narrator_lambda.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

# ----- Renderer Lambda Role -----

resource "aws_iam_role" "renderer_lambda" {
  name               = "${var.project_name}-renderer-lambda-${var.environment}"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume_role.json

  tags = merge(var.tags, {
    Name = "${var.project_name}-renderer-lambda-${var.environment}"
  })
}

resource "aws_iam_role_policy" "renderer_lambda" {
  name   = "${var.project_name}-renderer-lambda-policy-${var.environment}"
  role   = aws_iam_role.renderer_lambda.id
  policy = data.aws_iam_policy_document.renderer_lambda_policy.json
}

data "aws_iam_policy_document" "renderer_lambda_policy" {
  # CloudWatch Logs
  statement {
    actions = [
      "logs:CreateLogGroup",
      "logs:CreateLogStream",
      "logs:PutLogEvents"
    ]
    resources = ["arn:aws:logs:*:*:*"]
  }

  # S3: read from temp and parsed, write to output, delete temp files
  statement {
    actions = [
      "s3:GetObject"
    ]
    resources = [
      "${var.s3_bucket_arn}/temp/*",
      "${var.s3_bucket_arn}/parsed/*"
    ]
  }

  statement {
    actions = [
      "s3:PutObject"
    ]
    resources = ["${var.s3_bucket_arn}/output/*"]
  }

  statement {
    actions = [
      "s3:DeleteObject",
      "s3:ListBucket"
    ]
    resources = [
      "${var.s3_bucket_arn}/temp/*",
      var.s3_bucket_arn
    ]
  }
}

resource "aws_iam_role_policy_attachment" "renderer_lambda_basic" {
  role       = aws_iam_role.renderer_lambda.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

# ----- Step Functions Role -----

data "aws_iam_policy_document" "stepfunctions_assume_role" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["states.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "step_functions" {
  name               = "${var.project_name}-step-functions-${var.environment}"
  assume_role_policy = data.aws_iam_policy_document.stepfunctions_assume_role.json

  tags = merge(var.tags, {
    Name = "${var.project_name}-step-functions-${var.environment}"
  })
}

resource "aws_iam_role_policy" "step_functions" {
  name   = "${var.project_name}-step-functions-policy-${var.environment}"
  role   = aws_iam_role.step_functions.id
  policy = data.aws_iam_policy_document.step_functions_policy.json
}

data "aws_iam_policy_document" "step_functions_policy" {
  # Invoke Lambda functions
  statement {
    actions = [
      "lambda:InvokeFunction"
    ]
    resources = var.lambda_function_arns
  }

  # DynamoDB access to update project status
  statement {
    actions = [
      "dynamodb:UpdateItem"
    ]
    resources = [var.dynamodb_projects_table_arn]
  }

  # CloudWatch Logs for Step Functions execution history
  statement {
    actions = [
      "logs:CreateLogGroup",
      "logs:CreateLogStream",
      "logs:PutLogEvents",
      "logs:CreateLogDelivery",
      "logs:GetLogDelivery",
      "logs:UpdateLogDelivery",
      "logs:DeleteLogDelivery",
      "logs:ListLogDeliveries",
      "logs:PutResourcePolicy",
      "logs:DescribeResourcePolicies",
      "logs:DescribeLogGroups"
    ]
    resources = ["*"]
  }
}

# ----- API Gateway Authorizer Lambda Role -----

resource "aws_iam_role" "authorizer_lambda" {
  name               = "${var.project_name}-authorizer-lambda-${var.environment}"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume_role.json

  tags = merge(var.tags, {
    Name = "${var.project_name}-authorizer-lambda-${var.environment}"
  })
}

resource "aws_iam_role_policy" "authorizer_lambda" {
  name   = "${var.project_name}-authorizer-lambda-policy-${var.environment}"
  role   = aws_iam_role.authorizer_lambda.id
  policy = data.aws_iam_policy_document.authorizer_lambda_policy.json
}

data "aws_iam_policy_document" "authorizer_lambda_policy" {
  # CloudWatch Logs
  statement {
    actions = [
      "logs:CreateLogGroup",
      "logs:CreateLogStream",
      "logs:PutLogEvents"
    ]
    resources = ["arn:aws:logs:*:*:*"]
  }

  # DynamoDB: read sessions for token validation
  statement {
    actions = [
      "dynamodb:GetItem",
      "dynamodb:Query"
    ]
    resources = [var.dynamodb_sessions_table_arn]
  }
}

resource "aws_iam_role_policy_attachment" "authorizer_lambda_basic" {
  role       = aws_iam_role.authorizer_lambda.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}
