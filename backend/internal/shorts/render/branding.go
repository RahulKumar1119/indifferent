package render

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/rahul/indifferent/backend/internal/storage"
)

// DynamoGetter is the subset of the DynamoDB API used to resolve branding.
type DynamoGetter interface {
	GetItem(ctx context.Context, in *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
}

// projectBranding mirrors the stored "branding" JSON attribute (see
// models.Branding); only the logo key matters to the renderer.
type projectBranding struct {
	LogoKey string `json:"logoKey"`
}

// NewLogoFetcher returns a FetchLogo closure resolving the clip's brand logo:
// job (USER#/SHORTS#) → linked projectId → project (USER#/PROJECT#) →
// branding.logoKey → logo bytes from S3. It returns (nil, nil) when the job
// is unlinked or the project has no logo, and an error when a lookup or
// download fails (the renderer treats errors as skip-overlay, never fatal).
func NewLogoFetcher(db DynamoGetter, table, userID, jobID string, s3 storage.StorageClient, bucket string) func(ctx context.Context) ([]byte, error) {
	return func(ctx context.Context) ([]byte, error) {
		jobItem, err := db.GetItem(ctx, &dynamodb.GetItemInput{
			TableName: aws.String(table),
			Key: map[string]dbtypes.AttributeValue{
				"PK": &dbtypes.AttributeValueMemberS{Value: "USER#" + userID},
				"SK": &dbtypes.AttributeValueMemberS{Value: "SHORTS#" + jobID},
			},
			ConsistentRead: aws.Bool(true),
		})
		if err != nil {
			return nil, fmt.Errorf("lookup job: %w", err)
		}
		projectID := strAttr(jobItem.Item, "projectId")
		if projectID == "" {
			return nil, nil
		}

		projItem, err := db.GetItem(ctx, &dynamodb.GetItemInput{
			TableName: aws.String(table),
			Key: map[string]dbtypes.AttributeValue{
				"PK": &dbtypes.AttributeValueMemberS{Value: "USER#" + userID},
				"SK": &dbtypes.AttributeValueMemberS{Value: "PROJECT#" + projectID},
			},
			ConsistentRead: aws.Bool(true),
		})
		if err != nil {
			return nil, fmt.Errorf("lookup project: %w", err)
		}
		var branding projectBranding
		if raw := strAttr(projItem.Item, "branding"); raw == "" {
			return nil, nil
		} else if err := json.Unmarshal([]byte(raw), &branding); err != nil {
			return nil, fmt.Errorf("parse branding: %w", err)
		}
		if branding.LogoKey == "" {
			return nil, nil
		}

		logo, err := s3.GetObject(ctx, bucket, branding.LogoKey)
		if err != nil {
			return nil, fmt.Errorf("download logo: %w", err)
		}
		return logo, nil
	}
}

// strAttr reads a string attribute, tolerating missing items or attributes.
func strAttr(item map[string]dbtypes.AttributeValue, key string) string {
	if item == nil {
		return ""
	}
	if v, ok := item[key].(*dbtypes.AttributeValueMemberS); ok {
		return v.Value
	}
	return ""
}
