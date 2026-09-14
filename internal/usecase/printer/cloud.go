package printer

import (
	"fmt"

	printjobdomain "filamenttracker/internal/domain/printjob"
)

// CloudDevice is a vendor-neutral cloud printer snapshot.
type CloudDevice struct {
	Serial      string
	Name        string
	Model       string
	Online      bool
	PrintStatus string
	AccessCode  string
}

// CloudLoginResult is the outcome of a cloud credential login.
type CloudLoginResult struct {
	Token       string
	NeedsVerify bool
}

// CloudClient abstracts Bambu (or other) cloud authentication and device listing.
type CloudClient interface {
	Login(email, password, region string) (CloudLoginResult, error)
	Verify(email, code, region string) (token string, err error)
	ListDevices(token, region string) ([]CloudDevice, error)
	NormalizeRegion(region string) string
	MapPrintStatus(raw string) (status printjobdomain.Status, active bool)
	SendEmailCode(email, region string) error
}

// noopCloudClient is used when no CloudClient was injected (unit tests).
type noopCloudClient struct{}

func (noopCloudClient) Login(_, _, _ string) (CloudLoginResult, error) {
	return CloudLoginResult{}, fmt.Errorf("cloud client not configured")
}
func (noopCloudClient) Verify(_, _, _ string) (string, error) {
	return "", fmt.Errorf("cloud client not configured")
}
func (noopCloudClient) ListDevices(_, _ string) ([]CloudDevice, error) {
	return nil, fmt.Errorf("cloud client not configured")
}
func (noopCloudClient) NormalizeRegion(region string) string {
	r := region
	switch r {
	case "cn", "china", "China":
		return "cn"
	case "":
		return "us"
	default:
		return "us"
	}
}
func (noopCloudClient) MapPrintStatus(_ string) (printjobdomain.Status, bool) {
	return printjobdomain.StatusQueued, false
}
func (noopCloudClient) SendEmailCode(_, _ string) error {
	return fmt.Errorf("cloud client not configured")
}
