package bambu

import (
	printjobdomain "filamenttracker/internal/domain/printjob"
	printerusecase "filamenttracker/internal/usecase/printer"
)

// CloudClientAdapter implements printerusecase.CloudClient.
type CloudClientAdapter struct{}

func NewCloudClientAdapter() CloudClientAdapter {
	return CloudClientAdapter{}
}

func (CloudClientAdapter) Login(email, password, region string) (printerusecase.CloudLoginResult, error) {
	res, err := CloudLogin(email, password, region)
	if err != nil {
		return printerusecase.CloudLoginResult{}, err
	}
	return printerusecase.CloudLoginResult{Token: res.Token, NeedsVerify: res.NeedsVerify}, nil
}

func (CloudClientAdapter) Verify(email, code, region string) (string, error) {
	return CloudVerify(email, code, region)
}

func (CloudClientAdapter) ListDevices(token, region string) ([]printerusecase.CloudDevice, error) {
	devices, err := ListCloudDevices(token, region)
	if err != nil {
		return nil, err
	}
	out := make([]printerusecase.CloudDevice, 0, len(devices))
	for _, d := range devices {
		out = append(out, printerusecase.CloudDevice{
			Serial:      d.Serial,
			Name:        d.Name,
			Model:       d.Model,
			Online:      d.Online,
			PrintStatus: d.PrintStatus,
			AccessCode:  d.AccessCode,
		})
	}
	return out, nil
}

func (CloudClientAdapter) NormalizeRegion(region string) string {
	return NormalizeCloudRegion(region)
}

func (CloudClientAdapter) MapPrintStatus(raw string) (status printjobdomain.Status, active bool) {
	return MapCloudPrintStatus(raw)
}

func (CloudClientAdapter) SendEmailCode(email, region string) error {
	return CloudSendEmailCode(email, region)
}
