package base

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

func GetLatestCloudwatchLogs(region string, logGroupName string) ([]string, error) {
	ctx := context.Background()
	cfg, err := GetAWSConfig(ctx, region)
	if err != nil {
		return nil, err
	}
	svc := cloudwatchlogs.NewFromConfig(cfg)

	descending := true
	limit := int32(1)
	orderBy := types.OrderByLastEventTime
	streamsIn := &cloudwatchlogs.DescribeLogStreamsInput{
		Descending:   &descending,
		Limit:        &limit,
		LogGroupName: aws.String(logGroupName),
		OrderBy:      orderBy,
	}

	streamsOut, err := svc.DescribeLogStreams(ctx, streamsIn)
	if err != nil {
		return nil, err
	}

	if len(streamsOut.LogStreams) != 1 {
		return nil, fmt.Errorf("unable to find log groups. Found %d streams", len(streamsOut.LogStreams))
	}

	streamName := streamsOut.LogStreams[0].LogStreamName
	if streamName == nil {
		return nil, fmt.Errorf("Unable to find valid stream %+v", streamsOut.LogStreams[0])
	}

	eventIn := &cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  aws.String(logGroupName),
		LogStreamName: streamName,
	}

	eventOut, err := svc.GetLogEvents(ctx, eventIn)
	if err != nil {
		return nil, err
	}

	res := make([]string, 0, len(eventOut.Events))
	for _, event := range eventOut.Events {
		if event.Message == nil || *event.Message == "" {
			continue
		}
		res = append(res, *event.Message)
	}
	return res, nil
}
