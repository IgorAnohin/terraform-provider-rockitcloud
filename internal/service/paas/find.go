package paas

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/paas"
	"github.com/hashicorp/aws-sdk-go-base/v2/awsv1shim/v2/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
)

func FindServiceByID(conn *paas.PaaS, id string) (*paas.Service, error) {
	input := &paas.DescribeServiceInput{
		ServiceId: aws.String(id),
	}

	output, err := conn.DescribeService(input)

	if tfawserr.ErrCodeEquals(err, ServiceNotFoundCode) {
		return nil, &resource.NotFoundError{
			LastError:   err,
			LastRequest: input,
		}
	}

	if err != nil {
		return nil, err
	}

	if output == nil || output.Service == nil {
		return nil, tfresource.NewEmptyResultError(input)
	}

	return output.Service, nil
}

func FindBackupUsers(conn *paas.PaaS) ([]*paas.BackupUser, error) {
	input := &paas.ListBackupUsersInput{}

	output, err := conn.ListBackupUsers(input)

	if err != nil {
		return nil, err
	}

	if output == nil || output.Users == nil {
		return nil, tfresource.NewEmptyResultError(input)
	}

	return output.Users, nil
}

func FindBackupById(conn *paas.PaaS, id string) (*paas.Backup, error) {
	input := &paas.DescribeBackupInput{
		BackupId: aws.String(id),
	}

	output, err := conn.DescribeBackup(input)

	if err != nil {
		return nil, err
	}

	if output == nil || output.Backup == nil {
		return nil, tfresource.NewEmptyResultError(input)
	}

	return output.Backup, nil
}

func FindBackups(conn *paas.PaaS, serviceClass, serviceId, serviceType string) ([]*paas.Backup, error) {
	input := &paas.ListBackupsInput{}

	if serviceClass != "" {
		input.ServiceClass = aws.String(serviceClass)
	}

	if serviceId != "" {
		input.ServiceId = aws.String(serviceId)
	}

	if serviceType != "" {
		input.ServiceType = aws.String(serviceType)
	}

	output, err := conn.ListBackups(input)

	if err != nil {
		return nil, err
	}

	if output == nil || output.Backups == nil {
		return nil, tfresource.NewEmptyResultError(input)
	}

	return output.Backups, nil
}

func FindPrometheusNotificationChannelByID(ctx context.Context, conn *paas.PaaS, serviceID, channelID string) (*paas.NotificationChannel, error) {
	input := &paas.ListNotificationChannelsInput{
		ServiceId: aws.String(serviceID),
	}
	output, err := conn.ListNotificationChannelsWithContext(ctx, input)
	if err != nil {
		return nil, err
	}
	if output.NotificationChannels == nil {
		return nil, newPrometheusNotFoundError(input, fmt.Errorf("notification channel %q not found", channelID))
	}

	for _, item := range output.NotificationChannels {
		if aws.StringValue(item.Id) == channelID {
			return item, nil
		}
	}

	return nil, newPrometheusNotFoundError(input, fmt.Errorf("notification channel %q not found", channelID))
}

func FindPrometheusRouteByID(ctx context.Context, conn *paas.PaaS, serviceID, routeID string) (*paas.PrometheusRoute, error) {
	input := &paas.ListPrometheusRoutesInput{
		ServiceId: aws.String(serviceID),
	}
	output, err := conn.ListPrometheusRoutesWithContext(ctx, input)
	if err != nil {
		return nil, err
	}
	if output.PrometheusRoutes == nil {
		return nil, newPrometheusNotFoundError(input, fmt.Errorf("Prometheus route %q not found", routeID))
	}

	for _, item := range output.PrometheusRoutes {
		if aws.StringValue(item.Id) == routeID {
			return item, nil
		}
	}

	return nil, newPrometheusNotFoundError(input, fmt.Errorf("Prometheus route %q not found", routeID))
}

func FindPrometheusScrapeJobByID(ctx context.Context, conn *paas.PaaS, serviceID, jobID string) (*paas.PrometheusScrapeJob, error) {
	input := &paas.ListPrometheusScrapeJobsInput{
		ServiceId: aws.String(serviceID),
	}
	output, err := conn.ListPrometheusScrapeJobsWithContext(ctx, input)
	if err != nil {
		return nil, err
	}
	if output.PrometheusScrapeJobs == nil {
		return nil, newPrometheusNotFoundError(input, fmt.Errorf("Prometheus scrape job %q not found", jobID))
	}

	for _, item := range output.PrometheusScrapeJobs {
		if aws.StringValue(item.Id) == jobID {
			return item, nil
		}
	}

	return nil, newPrometheusNotFoundError(input, fmt.Errorf("Prometheus scrape job %q not found", jobID))
}

func newPrometheusNotFoundError(input interface{}, err error) error {
	return &resource.NotFoundError{
		LastError:   err,
		LastRequest: input,
	}
}

func isPrometheusNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	if awsErr, ok := err.(awserr.RequestFailure); ok && awsErr.StatusCode() == http.StatusNotFound {
		return true
	}

	if awsErr, ok := err.(awserr.Error); ok {
		return strings.Contains(strings.ToLower(awsErr.Code()), "notfound")
	}

	return false
}
