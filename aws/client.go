package aws

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cloudfronttypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

const (
	defaultRegion      = "us-east-1"
	defaultTailLimit   = int32(100)
	defaultPollBackoff = 2 * time.Second
)

type AWSClient struct {
	cfg awssdk.Config

	Lambda         *lambda.Client
	APIGateway     *apigateway.Client
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

type InvalidationResult struct {
	ID             string
	Status         string
	DistributionID string
	Path           string
}

type TailEvent struct {
	Line string
	Err  error
}

func NewClient(ctx context.Context, profile string, isLocalStack bool) (*AWSClient, error) {
	region := envOrDefault("AWS_REGION", envOrDefault("AWS_DEFAULT_REGION", defaultRegion))
	localStackEndpoint := envOrDefault("LOCALSTACK_ENDPOINT", "http://localhost:4566")

	options := []func(*config.LoadOptions) error{
		config.WithRegion(region),
	}
	if profile != "" {
		options = append(options, config.WithSharedConfigProfile(profile))
	}
	if isLocalStack {
		options = append(options, config.WithBaseEndpoint(localStackEndpoint))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return nil, err
	}

	return &AWSClient{
		cfg:            awsCfg,
		Lambda:         lambda.NewFromConfig(awsCfg),
		APIGateway:     apigateway.NewFromConfig(awsCfg),
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
	httpAPIs, err := c.listHTTPAPIs(ctx)
	if err != nil {
		return nil, err
	}

	restAPIs, err := c.listRESTAPIs(ctx)
	if err != nil {
		return nil, err
	}

	apis := append(httpAPIs, restAPIs...)
	sort.SliceStable(apis, func(i, j int) bool {
		if apis[i].Protocol == apis[j].Protocol {
			return apis[i].Name < apis[j].Name
		}
		return apis[i].Protocol < apis[j].Protocol
	})

	return apis, nil
}

func (c *AWSClient) ListLogGroups(ctx context.Context) ([]LogGroup, error) {
	var groups []LogGroup
	paginator := cloudwatchlogs.NewDescribeLogGroupsPaginator(c.CloudWatchLogs, &cloudwatchlogs.DescribeLogGroupsInput{})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, group := range page.LogGroups {
			groups = append(groups, LogGroup{
				Name:          awssdk.ToString(group.LogGroupName),
				StoredBytes:   awssdk.ToInt64(group.StoredBytes),
				RetentionDays: awssdk.ToInt32(group.RetentionInDays),
			})
		}
	}

	sort.SliceStable(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	return groups, nil
}

func (c *AWSClient) TailLogGroup(ctx context.Context, logGroupName string, out chan<- TailEvent) {
	defer close(out)

	startTime := time.Now().Add(-5 * time.Minute).UnixMilli()
	seen := map[string]struct{}{}
	ticker := time.NewTicker(defaultPollBackoff)
	defer ticker.Stop()

	for {
		lines, nextStart, err := c.fetchLogLines(ctx, logGroupName, startTime, seen)
		if err != nil {
			sendTailEvent(ctx, out, TailEvent{Err: err})
			return
		}
		startTime = nextStart

		for _, line := range lines {
			if !sendTailEvent(ctx, out, TailEvent{Line: line}) {
				return
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (c *AWSClient) ListDistributions(ctx context.Context) ([]Distribution, error) {
	var distributions []Distribution
	paginator := cloudfront.NewListDistributionsPaginator(c.CloudFront, &cloudfront.ListDistributionsInput{})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		if page.DistributionList == nil {
			continue
		}
		for _, dist := range page.DistributionList.Items {
			distributions = append(distributions, Distribution{
				ID:         awssdk.ToString(dist.Id),
				Status:     awssdk.ToString(dist.Status),
				DomainName: awssdk.ToString(dist.DomainName),
			})
		}
	}

	sort.SliceStable(distributions, func(i, j int) bool { return distributions[i].ID < distributions[j].ID })
	return distributions, nil
}

func (c *AWSClient) CreateInvalidation(ctx context.Context, distributionID, invalidationPath string) (InvalidationResult, error) {
	if distributionID == "" {
		return InvalidationResult{}, fmt.Errorf("distribution id is required")
	}
	if invalidationPath == "" {
		invalidationPath = "/*"
	}
	if !strings.HasPrefix(invalidationPath, "/") {
		invalidationPath = "/" + invalidationPath
	}

	quantity := int32(1)
	input := &cloudfront.CreateInvalidationInput{
		DistributionId: awssdk.String(distributionID),
		InvalidationBatch: &cloudfronttypes.InvalidationBatch{
			CallerReference: awssdk.String(fmt.Sprintf("lazyinfra-%d", time.Now().UnixNano())),
			Paths: &cloudfronttypes.Paths{
				Quantity: awssdk.Int32(quantity),
				Items:    []string{invalidationPath},
			},
		},
	}

	output, err := c.CloudFront.CreateInvalidation(ctx, input)
	if err != nil {
		return InvalidationResult{}, err
	}
	if output.Invalidation == nil {
		return InvalidationResult{}, fmt.Errorf("cloudfront returned no invalidation details")
	}

	return InvalidationResult{
		ID:             awssdk.ToString(output.Invalidation.Id),
		Status:         awssdk.ToString(output.Invalidation.Status),
		DistributionID: distributionID,
		Path:           invalidationPath,
	}, nil
}

func (c *AWSClient) listHTTPAPIs(ctx context.Context) ([]API, error) {
	var apis []API
	var nextToken *string

	for {
		page, err := c.APIGatewayV2.GetApis(ctx, &apigatewayv2.GetApisInput{
			MaxResults: awssdk.String("100"),
			NextToken:  nextToken,
		})
		if err != nil {
			return nil, err
		}
		for _, api := range page.Items {
			apiID := awssdk.ToString(api.ApiId)
			routes, err := c.listHTTPRoutes(ctx, apiID)
			if err != nil {
				return nil, fmt.Errorf("list routes for %s: %w", awssdk.ToString(api.Name), err)
			}
			apis = append(apis, API{
				Name:        awssdk.ToString(api.Name),
				Protocol:    string(api.ProtocolType),
				Description: awssdk.ToString(api.Description),
				Routes:      routes,
			})
		}
		if page.NextToken == nil || awssdk.ToString(page.NextToken) == "" {
			break
		}
		nextToken = page.NextToken
	}

	return apis, nil
}

func (c *AWSClient) listHTTPRoutes(ctx context.Context, apiID string) ([]Route, error) {
	var routes []Route
	var nextToken *string

	for {
		page, err := c.APIGatewayV2.GetRoutes(ctx, &apigatewayv2.GetRoutesInput{
			ApiId:      awssdk.String(apiID),
			MaxResults: awssdk.String("100"),
			NextToken:  nextToken,
		})
		if err != nil {
			return nil, err
		}
		for _, route := range page.Items {
			method, routePath := splitRouteKey(awssdk.ToString(route.RouteKey))
			target := awssdk.ToString(route.Target)
			lambdaName := target

			if integrationID := strings.TrimPrefix(target, "integrations/"); integrationID != "" && integrationID != target {
				integration, err := c.APIGatewayV2.GetIntegration(ctx, &apigatewayv2.GetIntegrationInput{
					ApiId:         awssdk.String(apiID),
					IntegrationId: awssdk.String(integrationID),
				})
				if err != nil {
					return nil, err
				}
				lambdaName = lambdaFromURI(awssdk.ToString(integration.IntegrationUri))
			}

			routes = append(routes, Route{
				Method:         method,
				Path:           routePath,
				LambdaFunction: lambdaName,
			})
		}
		if page.NextToken == nil || awssdk.ToString(page.NextToken) == "" {
			break
		}
		nextToken = page.NextToken
	}

	sortRoutes(routes)
	return routes, nil
}

func (c *AWSClient) listRESTAPIs(ctx context.Context) ([]API, error) {
	var apis []API
	paginator := apigateway.NewGetRestApisPaginator(c.APIGateway, &apigateway.GetRestApisInput{})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, api := range page.Items {
			apiID := awssdk.ToString(api.Id)
			routes, err := c.listRESTRoutes(ctx, apiID)
			if err != nil {
				return nil, fmt.Errorf("list REST resources for %s: %w", awssdk.ToString(api.Name), err)
			}
			apis = append(apis, API{
				Name:        awssdk.ToString(api.Name),
				Protocol:    "REST",
				Description: awssdk.ToString(api.Description),
				Routes:      routes,
			})
		}
	}

	return apis, nil
}

func (c *AWSClient) listRESTRoutes(ctx context.Context, apiID string) ([]Route, error) {
	var routes []Route
	paginator := apigateway.NewGetResourcesPaginator(c.APIGateway, &apigateway.GetResourcesInput{
		RestApiId: awssdk.String(apiID),
		Embed:     []string{"methods"},
		Limit:     awssdk.Int32(500),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, resource := range page.Items {
			resourcePath := awssdk.ToString(resource.Path)
			for method := range resource.ResourceMethods {
				integration, err := c.APIGateway.GetIntegration(ctx, &apigateway.GetIntegrationInput{
					RestApiId:  awssdk.String(apiID),
					ResourceId: awssdk.String(awssdk.ToString(resource.Id)),
					HttpMethod: awssdk.String(method),
				})
				lambdaName := ""
				if err == nil {
					lambdaName = lambdaFromURI(awssdk.ToString(integration.Uri))
				}

				routes = append(routes, Route{
					Method:         method,
					Path:           resourcePath,
					LambdaFunction: lambdaName,
				})
			}
		}
	}

	sortRoutes(routes)
	return routes, nil
}

func (c *AWSClient) fetchLogLines(ctx context.Context, logGroupName string, startTime int64, seen map[string]struct{}) ([]string, int64, error) {
	paginator := cloudwatchlogs.NewFilterLogEventsPaginator(c.CloudWatchLogs, &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName: awssdk.String(logGroupName),
		StartTime:    awssdk.Int64(startTime),
		Limit:        awssdk.Int32(defaultTailLimit),
	})

	var lines []string
	nextStart := startTime
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, startTime, err
		}
		for _, event := range page.Events {
			eventID := awssdk.ToString(event.EventId)
			if _, ok := seen[eventID]; eventID != "" && ok {
				continue
			}
			if eventID != "" {
				seen[eventID] = struct{}{}
			}

			timestamp := awssdk.ToInt64(event.Timestamp)
			if timestamp >= nextStart {
				nextStart = timestamp + 1
			}
			lines = append(lines, fmt.Sprintf("%s %s", formatMillis(timestamp), awssdk.ToString(event.Message)))
		}
	}

	return lines, nextStart, nil
}

func sendTailEvent(ctx context.Context, out chan<- TailEvent, event TailEvent) bool {
	select {
	case <-ctx.Done():
		return false
	case out <- event:
		return true
	}
}

func splitRouteKey(routeKey string) (string, string) {
	if routeKey == "" {
		return "-", "/"
	}
	if routeKey == "$default" {
		return "ANY", "$default"
	}
	parts := strings.Fields(routeKey)
	if len(parts) < 2 {
		return "-", routeKey
	}
	return parts[0], parts[1]
}

func lambdaFromURI(uri string) string {
	if uri == "" {
		return ""
	}
	if before, after, ok := strings.Cut(uri, "/functions/"); ok {
		functionARN, _, _ := strings.Cut(after, "/invocations")
		return functionNameFromARN(functionARN)
	} else if strings.Contains(before, ":function:") {
		return functionNameFromARN(before)
	}
	parsed, err := url.Parse(uri)
	if err == nil && parsed.Path != "" {
		segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		for i, segment := range segments {
			if segment == "functions" && i+1 < len(segments) {
				return functionNameFromARN(segments[i+1])
			}
		}
	}
	return functionNameFromARN(path.Base(uri))
}

func functionNameFromARN(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parts := strings.Split(value, ":")
	if len(parts) >= 7 && parts[5] == "function" {
		return parts[6]
	}
	if len(parts) >= 2 && parts[len(parts)-2] == "function" {
		return parts[len(parts)-1]
	}
	return strings.TrimSuffix(value, "/invocations")
}

func sortRoutes(routes []Route) {
	sort.SliceStable(routes, func(i, j int) bool {
		if routes[i].Path == routes[j].Path {
			return routes[i].Method < routes[j].Method
		}
		return routes[i].Path < routes[j].Path
	})
}

func formatMillis(ms int64) string {
	if ms <= 0 {
		return time.Now().Format(time.RFC3339)
	}
	return time.UnixMilli(ms).Format(time.RFC3339)
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
