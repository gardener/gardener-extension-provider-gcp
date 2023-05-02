package client

import (
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/api/compute/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	pollInterval = 10 * time.Second
)

// Wait waits for async operations to complete.
func (c *computeClient) wait(ctx context.Context, op *compute.Operation) error {
	return wait.PollImmediateUntil(pollInterval, c.waitOperation(op), ctx.Done())
}

func (c *computeClient) QueryOperation(op *compute.Operation) (*compute.Operation, error) {
	switch {
	case op.Zone != "":
		return c.service.ZoneOperations.Get(c.projectID, parseResourceName(op.Zone), op.Name).Do()
	case op.Region != "":
		return c.service.RegionOperations.Get(c.projectID, parseResourceName(op.Region), op.Name).Do()
	default:
		return c.service.GlobalOperations.Get(c.projectID, op.Name).Do()
	}
}

func (c *computeClient) waitOperation(op *compute.Operation) func() (bool, error) {
	return func() (bool, error) {
		result, err := c.QueryOperation(op)
		if err != nil {
			return false, fmt.Errorf("failed to query operation [Name=%s]: %s", op.Name, err)
		}

		if result.Status == "DONE" {
			if result.Error != nil {
				var errors []string
				for _, e := range result.Error.Errors {
					errors = append(errors, e.Message)
				}
				return false, fmt.Errorf("operation %q failed with error(s): %s", op.Name, strings.Join(errors, ", "))
			}
			return true, nil
		}

		return false, nil
	}
}

func parseResourceName(url string) string {
	if len(url) == 0 {
		return ""
	}

	segments := strings.Split(url, "/")
	if len(segments) > 0 {
		return segments[len(segments)-1]
	}

	return ""
}
