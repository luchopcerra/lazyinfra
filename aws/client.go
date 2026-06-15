package aws

import (
	"context"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

const defaultRegion = "us-east-1"

type AWSClient struct {
	cfg awssdk.Config

	Lambda         *lambda.Client
	APIGatewayV2   *apigatewayv2.Client
	CloudWatchLogs *cloudwatchlogs.Client
	CloudFront     *cloudfront.Client
}

type API struct {
	Name        string
	Protocol    string
	Routes      []Route
	Description string
}

type Route struct {
	Method         string
	Path           string
	LambdaFunction string
}

type LogGroup struct {
	Name          string
	StoredBytes   int64
	RetentionDays int32
}

type Distribution struct {
	ID         string
	Status     string
	DomainName string
}

func NewClient(ctx context.Context, profile string, isLocalStack bool) (*AWSClient, error) {
	region := defaultRegion
	options := []func(*config.LoadOptions) error{
		config.WithRegion(region),
	}
	if profile != "" {
		options = append(options, config.WithSharedConfigProfile(profile))
	}
	if isLocalStack {
		options = append(options, config.WithBaseEndpoint("http://localhost:4566"))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return nil, err
	}

	return &AWSClient{
		cfg:            awsCfg,
		Lambda:         lambda.NewFromConfig(awsCfg),
		APIGatewayV2:   apigatewayv2.NewFromConfig(awsCfg),
		CloudWatchLogs: cloudwatchlogs.NewFromConfig(awsCfg),
		CloudFront:     cloudfront.NewFromConfig(awsCfg),
	}, nil
}

func (c *AWSClient) FetchLambdas(ctx context.Context) ([]types.FunctionConfiguration, error) {
	var functions []types.FunctionConfiguration
	paginator := lambda.NewListFunctionsPaginator(c.Lambda, &lambda.ListFunctionsInput{})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		functions = append(functions, page.Functions...)
	}

	return functions, nil
}

func (c *AWSClient) ListAPIs(ctx context.Context) ([]API, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(220 * time.Millisecond):
	}

	return []API{
		{
			Name:     "backend-dev-http",
			Protocol: "HTTP",
			Routes: []Route{
				{Method: "GET", Path: "/users", LambdaFunction: "users-list"},
				{Method: "POST", Path: "/users", LambdaFunction: "users-create"},
				{Method: "POST", Path: "/orders", LambdaFunction: "orders-process"},
			},
		},
		{
			Name:     "legacy-admin-rest",
			Protocol: "REST",
			Routes: []Route{
				{Method: "GET", Path: "/admin/reports", LambdaFunction: "reports-generate"},
				{Method: "DELETE", Path: "/admin/cache", LambdaFunction: "cache-purge"},
			},
		},
	}, nil
}

func (c *AWSClient) ListLogGroups(ctx context.Context) ([]LogGroup, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(160 * time.Millisecond):
	}

	return []LogGroup{
		{Name: "/aws/lambda/users-create", StoredBytes: 834912, RetentionDays: 14},
		{Name: "/aws/lambda/orders-process", StoredBytes: 2238912, RetentionDays: 30},
		{Name: "/aws/apigateway/backend-dev-http", StoredBytes: 549222, RetentionDays: 7},
	}, nil
}

func (c *AWSClient) TailLogSample(ctx context.Context) ([]string, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(350 * time.Millisecond):
	}

	return []string{
		"2026-06-14T13:01:22Z INFO request started route=/orders",
		"2026-06-14T13:01:23Z INFO lambda invoked function=orders-process",
		"2026-06-14T13:01:23Z ERROR validation failed: missing customerId",
		"2026-06-14T13:01:24Z Exception: retry scheduled for message id=evt_123",
	}, nil
}

func (c *AWSClient) ListDistributions(ctx context.Context) ([]Distribution, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(200 * time.Millisecond):
	}

	return []Distribution{
		{ID: "E1DEV12345", Status: "Deployed", DomainName: "d111111abcdef8.cloudfront.net"},
		{ID: "E2STAGE6789", Status: "InProgress", DomainName: "d222222abcdef8.cloudfront.net"},
	}, nil
}
