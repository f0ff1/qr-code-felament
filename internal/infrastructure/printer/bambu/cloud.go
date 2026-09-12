package bambu

import (
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"

	printjobdomain "filamenttracker/internal/domain/printjob"
	printjobusecase "filamenttracker/internal/usecase/printjob"

	bambicloud "github.com/torbenconto/bambulabs_cloud_api"
	"github.com/torbenconto/bambulabs_cloud_api/state"
)

func MapCloudRegion(raw string) bambicloud.Region {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "cn", "china":
		return bambicloud.China
	default:
		// Europe / NA / APAC all use api.bambulab.com in this library.
		return bambicloud.NorthAmerica
	}
}

func NormalizeCloudRegion(raw string) string {
	if MapCloudRegion(raw) == bambicloud.China {
		return "cn"
	}
	return "us"
}

type CloudLoginResult struct {
	Token       string
	NeedsVerify bool
}

type CloudDevice struct {
	Serial      string
	Name        string
	Model       string
	Online      bool
	PrintStatus string
	AccessCode  string
}

func CloudLogin(email, password, region string) (CloudLoginResult, error) {
	email = strings.TrimSpace(email)
	password = strings.TrimSpace(password)
	if email == "" || password == "" {
		return CloudLoginResult{}, fmt.Errorf("cloud email and password are required")
	}
	client := bambicloud.NewClient(&bambicloud.Config{
		Region:   MapCloudRegion(region),
		Email:    email,
		Password: password,
	})
	token, err := client.Login()
	if err != nil {
		return CloudLoginResult{}, err
	}
	if token == "" {
		return CloudLoginResult{NeedsVerify: true}, nil
	}
	return CloudLoginResult{Token: token}, nil
}

func CloudVerify(email, code, region string) (string, error) {
	email = strings.TrimSpace(email)
	code = strings.TrimSpace(code)
	if email == "" || code == "" {
		return "", fmt.Errorf("cloud email and verification code are required")
	}
	client := bambicloud.NewClient(&bambicloud.Config{
		Region: MapCloudRegion(region),
		Email:  email,
	})
	token, err := client.SubmitVerificationCode(code)
	if err != nil {
		return "", err
	}
	if token == "" {
		return "", fmt.Errorf("empty cloud token after verification")
	}
	return token, nil
}

func ListCloudDevices(token, region string) ([]CloudDevice, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("cloud token is required")
	}
	client := newCloudClient(token, region)
	devices, err := client.ListDevices()
	if err != nil {
		return nil, err
	}
	out := make([]CloudDevice, 0, len(devices))
	for _, d := range devices {
		model := strings.TrimSpace(d.DevProductName)
		if model == "" {
			model = strings.TrimSpace(d.DevModelName)
		}
		name := strings.TrimSpace(d.Name)
		if name == "" {
			name = d.DevID
		}
		out = append(out, CloudDevice{
			Serial:      strings.TrimSpace(d.DevID),
			Name:        name,
			Model:       model,
			Online:      d.Online,
			PrintStatus: strings.TrimSpace(d.PrintStatus),
			AccessCode:  strings.TrimSpace(d.DevAccessCode),
		})
	}
	return out, nil
}

func newCloudClient(token, region string) *bambicloud.Client {
	return bambicloud.NewClientWithToken(MapCloudRegion(region), strings.TrimSpace(token))
}

func snapshotFromCloudData(data bambicloud.Data, weightHint int, materialHint, colorHint string) (printjobusecase.BambuSnapshot, bool) {
	status, active := mapCloudGcodeState(data.GcodeState)
	fileName := strings.TrimSpace(data.SubtaskName)
	if fileName == "" {
		fileName = strings.TrimSpace(data.GcodeFile)
	}
	fileName = filepath.Base(fileName)

	taskID := ""
	if data.TaskID != 0 {
		taskID = strconv.Itoa(data.TaskID)
	} else if data.SubtaskID != 0 {
		taskID = strconv.Itoa(data.SubtaskID)
	}
	if taskID == "" && fileName != "" {
		taskID = fileName
	}

	material := materialHint
	if material == "" {
		material = data.VtTray.TrayType
	}
	color := colorHint
	if color == "" && (data.VtTray.TrayColor.R != 0 || data.VtTray.TrayColor.G != 0 || data.VtTray.TrayColor.B != 0) {
		color = fmt.Sprintf("%02X%02X%02X", data.VtTray.TrayColor.R, data.VtTray.TrayColor.G, data.VtTray.TrayColor.B)
	}

	return printjobusecase.BambuSnapshot{
		ExternalTaskID:  taskID,
		FileName:        fileName,
		Progress:        float64(data.PrintPercentDone),
		Status:          status,
		RemainingMin:    data.RemainingPrintTime,
		MaterialHint:    material,
		ColorHint:       color,
		EstimatedWeight: weightHint,
	}, active
}

func snapshotFromCloudDevice(device bambicloud.Device, tasks *bambicloud.GetTasksResponse) (printjobusecase.BambuSnapshot, bool) {
	status, active := mapCloudPrintStatus(device.PrintStatus)
	fileName := ""
	// Stable per-device active job id; refined with cloud task id when available.
	taskID := "cloud-" + strings.TrimSpace(device.DevID)
	weight := 0
	material := ""
	color := ""

	if tasks != nil {
		for _, hit := range tasks.Hits {
			if hit.DeviceID != "" && !strings.EqualFold(hit.DeviceID, device.DevID) {
				continue
			}
			if hit.Title != "" {
				fileName = hit.Title
			}
			if hit.ID != 0 {
				taskID = "cloud-task-" + strconv.Itoa(hit.ID)
			}
			if hit.Weight > 0 {
				weight = int(math.Round(hit.Weight))
			}
			for _, ams := range hit.AMSDetailMapping {
				if material == "" && ams.FilamentType != "" {
					material = ams.FilamentType
				}
				if color == "" && ams.SourceColor != "" {
					color = ams.SourceColor
				}
				if weight == 0 && ams.Weight > 0 {
					weight = int(math.Round(ams.Weight))
				}
			}
			break
		}
	}
	if fileName == "" {
		fileName = device.Name
	}
	if fileName == "" {
		fileName = device.DevID
	}

	progress := 0.0
	if status == printjobdomain.StatusPrinting || status == printjobdomain.StatusPaused {
		progress = 1
	} else if status == printjobdomain.StatusCompleted {
		progress = 100
	}

	return printjobusecase.BambuSnapshot{
		ExternalTaskID:  taskID,
		FileName:        fileName,
		Progress:        progress,
		Status:          status,
		MaterialHint:    material,
		ColorHint:       color,
		EstimatedWeight: weight,
	}, active
}

func mergeCloudSnapshots(rest printjobusecase.BambuSnapshot, restActive bool, mqtt printjobusecase.BambuSnapshot, mqttActive bool) (printjobusecase.BambuSnapshot, bool) {
	snap := rest
	active := restActive || mqttActive

	if mqtt.FileName != "" && mqtt.FileName != "." {
		snap.FileName = mqtt.FileName
	}
	if mqtt.ExternalTaskID != "" {
		snap.ExternalTaskID = mqtt.ExternalTaskID
	}
	if mqtt.Progress > snap.Progress {
		snap.Progress = mqtt.Progress
	}
	if mqtt.RemainingMin > 0 {
		snap.RemainingMin = mqtt.RemainingMin
	}
	if mqtt.MaterialHint != "" {
		snap.MaterialHint = mqtt.MaterialHint
	}
	if mqtt.ColorHint != "" {
		snap.ColorHint = mqtt.ColorHint
	}
	if mqtt.EstimatedWeight > 0 {
		snap.EstimatedWeight = mqtt.EstimatedWeight
	}

	// Cloud REST uses ACTIVE; MQTT uses RUNNING. Prefer whichever is active.
	if restActive {
		snap.Status = rest.Status
	} else if mqttActive {
		snap.Status = mqtt.Status
	} else if mqtt.Status != "" {
		snap.Status = mqtt.Status
	}
	return snap, active
}

func mapCloudGcodeState(raw state.GcodeState) (printjobdomain.Status, bool) {
	return mapGcodeState(string(raw))
}

func mapCloudPrintStatus(raw string) (printjobdomain.Status, bool) {
	return MapCloudPrintStatus(raw)
}

// MapCloudPrintStatus maps Bambu Cloud device print_status values.
func MapCloudPrintStatus(raw string) (printjobdomain.Status, bool) {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "ACTIVE", "RUNNING", "PRINTING", "PREPARE", "SLICING", "BUSY", "WORKING":
		return printjobdomain.StatusPrinting, true
	case "PAUSE", "PAUSED":
		return printjobdomain.StatusPaused, true
	case "FINISH", "FINISHED", "SUCCESS":
		return printjobdomain.StatusCompleted, true
	case "FAILED", "FAILURE", "ERROR":
		return printjobdomain.StatusFailed, true
	default:
		return printjobdomain.StatusQueued, false
	}
}

func findCloudDevice(devices []bambicloud.Device, serial string) (bambicloud.Device, bool) {
	serial = strings.TrimSpace(serial)
	for _, d := range devices {
		if strings.EqualFold(d.DevID, serial) {
			return d, true
		}
	}
	return bambicloud.Device{}, false
}
