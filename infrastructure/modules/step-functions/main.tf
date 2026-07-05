# Step Functions state machine for the TXT-to-Video pipeline

resource "aws_sfn_state_machine" "pipeline" {
  name     = "${var.project_name}-pipeline-${var.environment}"
  role_arn = var.step_functions_role_arn

  definition = jsonencode({
    Comment = "TXT-to-Video conversion pipeline: Parse → Generate Slides → Narrate → Render"
    StartAt = "UpdateStatus_Parsing"
    States = {
      UpdateStatus_Parsing = {
        Type     = "Task"
        Resource = "arn:aws:states:::dynamodb:updateItem"
        Parameters = {
          TableName = var.dynamodb_projects_table_name
          Key = {
            "PK" = { "S.$" = "$.userId" }
            "SK" = { "S.$" = "$.projectSK" }
          }
          UpdateExpression = "SET #status = :status, #updatedAt = :updatedAt"
          ExpressionAttributeNames = {
            "#status"    = "status"
            "#updatedAt" = "updatedAt"
          }
          ExpressionAttributeValues = {
            ":status"    = { "S" = "parsing" }
            ":updatedAt" = { "S.$" = "$$.State.EnteredTime" }
          }
        }
        ResultPath = null
        Next       = "ParseTXT"
      }

      ParseTXT = {
        Type     = "Task"
        Resource = var.parser_lambda_arn
        Parameters = {
          "projectId.$" = "$.projectId"
          "userId.$"    = "$.userId"
          "s3Key.$"     = "$.txtKey"
        }
        Retry = [
          {
            ErrorEquals     = ["States.TaskFailed", "States.Timeout"]
            IntervalSeconds = 5
            MaxAttempts     = 2
            BackoffRate     = 2.0
          }
        ]
        Catch = [
          {
            ErrorEquals = ["States.ALL"]
            ResultPath  = "$.error"
            Next        = "MarkFailed"
          }
        ]
        ResultPath = "$.parserOutput"
        Next       = "UpdateStatus_GeneratingSlides"
      }

      UpdateStatus_GeneratingSlides = {
        Type     = "Task"
        Resource = "arn:aws:states:::dynamodb:updateItem"
        Parameters = {
          TableName = var.dynamodb_projects_table_name
          Key = {
            "PK" = { "S.$" = "$.userId" }
            "SK" = { "S.$" = "$.projectSK" }
          }
          UpdateExpression = "SET #status = :status, #updatedAt = :updatedAt, #jsonKey = :jsonKey, #questionCount = :questionCount"
          ExpressionAttributeNames = {
            "#status"        = "status"
            "#updatedAt"     = "updatedAt"
            "#jsonKey"       = "jsonKey"
            "#questionCount" = "questionCount"
          }
          ExpressionAttributeValues = {
            ":status"        = { "S" = "generating_slides" }
            ":updatedAt"     = { "S.$" = "$$.State.EnteredTime" }
            ":jsonKey"       = { "S.$" = "$.parserOutput.s3Key" }
            ":questionCount" = { "N.$" = "States.Format('{}', $.parserOutput.questions)" }
          }
        }
        ResultPath = null
        Next       = "GenerateSlides"
      }

      GenerateSlides = {
        Type     = "Task"
        Resource = var.slidegen_lambda_arn
        Parameters = {
          "projectId.$" = "$.projectId"
          "jsonKey.$"   = "$.parserOutput.s3Key"
          "template.$"  = "$.template"
        }
        Retry = [
          {
            ErrorEquals     = ["States.TaskFailed", "States.Timeout"]
            IntervalSeconds = 10
            MaxAttempts     = 2
            BackoffRate     = 2.0
          }
        ]
        Catch = [
          {
            ErrorEquals = ["States.ALL"]
            ResultPath  = "$.error"
            Next        = "MarkFailed"
          }
        ]
        ResultPath = "$.slideGenOutput"
        Next       = "UpdateStatus_Narrating"
      }

      UpdateStatus_Narrating = {
        Type     = "Task"
        Resource = "arn:aws:states:::dynamodb:updateItem"
        Parameters = {
          TableName = var.dynamodb_projects_table_name
          Key = {
            "PK" = { "S.$" = "$.userId" }
            "SK" = { "S.$" = "$.projectSK" }
          }
          UpdateExpression = "SET #status = :status, #updatedAt = :updatedAt"
          ExpressionAttributeNames = {
            "#status"    = "status"
            "#updatedAt" = "updatedAt"
          }
          ExpressionAttributeValues = {
            ":status"    = { "S" = "narrating" }
            ":updatedAt" = { "S.$" = "$$.State.EnteredTime" }
          }
        }
        ResultPath = null
        Next       = "Narrate"
      }

      Narrate = {
        Type     = "Task"
        Resource = var.narrator_lambda_arn
        Parameters = {
          "projectId.$" = "$.projectId"
          "jsonKey.$"   = "$.parserOutput.s3Key"
          "voice.$"     = "$.voice"
        }
        Retry = [
          {
            ErrorEquals     = ["States.TaskFailed", "States.Timeout"]
            IntervalSeconds = 10
            MaxAttempts     = 2
            BackoffRate     = 2.0
          }
        ]
        Catch = [
          {
            ErrorEquals = ["States.ALL"]
            ResultPath  = "$.error"
            Next        = "MarkFailed"
          }
        ]
        ResultPath = "$.narratorOutput"
        Next       = "UpdateStatus_Rendering"
      }

      UpdateStatus_Rendering = {
        Type     = "Task"
        Resource = "arn:aws:states:::dynamodb:updateItem"
        Parameters = {
          TableName = var.dynamodb_projects_table_name
          Key = {
            "PK" = { "S.$" = "$.userId" }
            "SK" = { "S.$" = "$.projectSK" }
          }
          UpdateExpression = "SET #status = :status, #updatedAt = :updatedAt"
          ExpressionAttributeNames = {
            "#status"    = "status"
            "#updatedAt" = "updatedAt"
          }
          ExpressionAttributeValues = {
            ":status"    = { "S" = "rendering" }
            ":updatedAt" = { "S.$" = "$$.State.EnteredTime" }
          }
        }
        ResultPath = null
        Next       = "Render"
      }

      Render = {
        Type     = "Task"
        Resource = var.renderer_lambda_arn
        Parameters = {
          "projectId.$" = "$.projectId"
          "slideKeys.$" = "$.slideGenOutput.slideKeys"
          "audioKeys.$" = "$.narratorOutput.audioKeys"
          "jsonKey.$"   = "$.parserOutput.s3Key"
        }
        Retry = [
          {
            ErrorEquals     = ["States.TaskFailed", "States.Timeout"]
            IntervalSeconds = 30
            MaxAttempts     = 2
            BackoffRate     = 2.0
          }
        ]
        Catch = [
          {
            ErrorEquals = ["States.ALL"]
            ResultPath  = "$.error"
            Next        = "MarkFailed"
          }
        ]
        ResultPath = "$.rendererOutput"
        Next       = "MarkCompleted"
      }

      MarkCompleted = {
        Type     = "Task"
        Resource = "arn:aws:states:::dynamodb:updateItem"
        Parameters = {
          TableName = var.dynamodb_projects_table_name
          Key = {
            "PK" = { "S.$" = "$.userId" }
            "SK" = { "S.$" = "$.projectSK" }
          }
          UpdateExpression = "SET #status = :status, #updatedAt = :updatedAt, #completedAt = :completedAt, #videoKey = :videoKey, #thumbnailKey = :thumbnailKey"
          ExpressionAttributeNames = {
            "#status"       = "status"
            "#updatedAt"    = "updatedAt"
            "#completedAt"  = "completedAt"
            "#videoKey"     = "videoKey"
            "#thumbnailKey" = "thumbnailKey"
          }
          ExpressionAttributeValues = {
            ":status"       = { "S" = "completed" }
            ":updatedAt"    = { "S.$" = "$$.State.EnteredTime" }
            ":completedAt"  = { "S.$" = "$$.State.EnteredTime" }
            ":videoKey"     = { "S.$" = "$.rendererOutput.videoKey" }
            ":thumbnailKey" = { "S.$" = "$.rendererOutput.thumbnailKey" }
          }
        }
        End = true
      }

      MarkFailed = {
        Type     = "Task"
        Resource = "arn:aws:states:::dynamodb:updateItem"
        Parameters = {
          TableName = var.dynamodb_projects_table_name
          Key = {
            "PK" = { "S.$" = "$.userId" }
            "SK" = { "S.$" = "$.projectSK" }
          }
          UpdateExpression = "SET #status = :status, #updatedAt = :updatedAt, #error = :error"
          ExpressionAttributeNames = {
            "#status"    = "status"
            "#updatedAt" = "updatedAt"
            "#error"     = "error"
          }
          ExpressionAttributeValues = {
            ":status"    = { "S" = "failed" }
            ":updatedAt" = { "S.$" = "$$.State.EnteredTime" }
            ":error"     = { "S.$" = "States.Format('{}', $.error)" }
          }
        }
        End = true
      }
    }
  })

  tags = merge(var.tags, {
    Name = "${var.project_name}-pipeline-${var.environment}"
  })
}
