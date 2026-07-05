# Lambda functions for the TXT-to-Video pipeline

# ----- API Lambda -----

resource "aws_lambda_function" "api" {
  function_name = "${var.project_name}-api-${var.environment}"
  role          = var.api_lambda_role_arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  memory_size   = 256
  timeout       = 30

  filename         = var.api_lambda_zip_path
  source_code_hash = filebase64sha256(var.api_lambda_zip_path)

  environment {
    variables = {
      ENVIRONMENT             = var.environment
      DYNAMODB_USERS_TABLE    = var.dynamodb_users_table_name
      DYNAMODB_PROJECTS_TABLE = var.dynamodb_projects_table_name
      DYNAMODB_SESSIONS_TABLE = var.dynamodb_sessions_table_name
      S3_BUCKET               = var.s3_bucket_name
      STEP_FUNCTIONS_ARN      = var.step_functions_arn
      GOOGLE_CLIENT_ID        = var.google_client_id
      GOOGLE_CLIENT_SECRET    = var.google_client_secret
      JWT_SECRET              = var.jwt_secret
      FRONTEND_DOMAIN         = var.frontend_domain
    }
  }

  tags = merge(var.tags, {
    Name = "${var.project_name}-api-${var.environment}"
  })
}

# ----- Parser Lambda -----

resource "aws_lambda_function" "parser" {
  function_name = "${var.project_name}-parser-${var.environment}"
  role          = var.parser_lambda_role_arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  memory_size   = 256
  timeout       = 60

  filename         = var.parser_lambda_zip_path
  source_code_hash = filebase64sha256(var.parser_lambda_zip_path)

  environment {
    variables = {
      ENVIRONMENT = var.environment
      S3_BUCKET   = var.s3_bucket_name
    }
  }

  tags = merge(var.tags, {
    Name = "${var.project_name}-parser-${var.environment}"
  })
}

# ----- Slide Generator Lambda -----

resource "aws_lambda_function" "slidegen" {
  function_name = "${var.project_name}-slidegen-${var.environment}"
  role          = var.slidegen_lambda_role_arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  memory_size   = 1024
  timeout       = 300

  filename         = var.slidegen_lambda_zip_path
  source_code_hash = filebase64sha256(var.slidegen_lambda_zip_path)

  environment {
    variables = {
      ENVIRONMENT = var.environment
      S3_BUCKET   = var.s3_bucket_name
    }
  }

  tags = merge(var.tags, {
    Name = "${var.project_name}-slidegen-${var.environment}"
  })
}

# ----- Narrator Lambda -----

resource "aws_lambda_function" "narrator" {
  function_name = "${var.project_name}-narrator-${var.environment}"
  role          = var.narrator_lambda_role_arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  memory_size   = 256
  timeout       = 300

  filename         = var.narrator_lambda_zip_path
  source_code_hash = filebase64sha256(var.narrator_lambda_zip_path)

  environment {
    variables = {
      ENVIRONMENT = var.environment
      S3_BUCKET   = var.s3_bucket_name
    }
  }

  tags = merge(var.tags, {
    Name = "${var.project_name}-narrator-${var.environment}"
  })
}

# ----- Renderer Lambda (Container Image) -----

resource "aws_lambda_function" "renderer" {
  function_name = "${var.project_name}-renderer-${var.environment}"
  role          = var.renderer_lambda_role_arn
  package_type  = "Image"
  image_uri     = var.renderer_image_uri
  memory_size   = 3008
  timeout       = 900

  environment {
    variables = {
      ENVIRONMENT = var.environment
      S3_BUCKET   = var.s3_bucket_name
    }
  }

  tags = merge(var.tags, {
    Name = "${var.project_name}-renderer-${var.environment}"
  })
}
