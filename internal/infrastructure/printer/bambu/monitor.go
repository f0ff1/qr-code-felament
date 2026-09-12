package bambu

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	printerdomain "filamenttracker/internal/domain/printer"
	printjobdomain "filamenttracker/internal/domain/printjob"
	printjobusecase "filamenttracker/internal/usecase/printjob"

	"github.com/google/uuid"
	bambulabs "github.com/torbenconto/bambulabs_api"
	bambicloud "github.com/torbenconto/bambulabs_cloud_api"
)

type PrinterStore interface {
	List(ctx context.Context) ([]printerdomain.Printer, error)
	Update(ctx context.Context, p printerdomain.Printer) error
}

type JobSyncer interface {
	SyncFromBambu(ctx context.Context, printer printerdomain.Printer, snap printjobusecase.BambuSnapshot) (printjobdomain.PrintJob, bool, error)
}

type EventPublisher interface {
	Publish(eventType, message string, payload map[string]any)
}

type cloudSession struct {
	client *bambicloud.Client
	pool   *bambicloud.PrinterPool
	token  string
}

type Monitor struct {
	client   *bambulabs.Client
	printers PrinterStore
	jobs     JobSyncer
	notifier EventPublisher
	interval time.Duration

	mu     sync.Mutex
	known  map[string]uuid.UUID
	cancel context.CancelFunc

	cloudMu       sync.Mutex
	cloudSessions map[string]*cloudSession
}

type printTelemetry struct {
	Print struct {
		GcodeFile       string `json:"gcode_file"`
		GcodeStartTime  string `json:"gcode_start_time"`
		GcodeState      string `json:"gcode_state"`
		McPercent       int    `json:"mc_percent"`
		McRemainingTime int    `json:"mc_remaining_time"`
		SubtaskID       string `json:"subtask_id"`
		SubtaskName     string `json:"subtask_name"`
		TaskID          string `json:"task_id"`
		VtTray          struct {
			TrayColor string `json:"tray_color"`
			TrayType  string `json:"tray_type"`
		} `json:"vt_tray"`
		Ams struct {
			TrayNow string `json:"tray_now"`
			Ams     []struct {
				Tray []struct {
					ID        string `json:"id"`
					TrayColor string `json:"tray_color"`
					TrayType  string `json:"tray_type"`
				} `json:"tray"`
			} `json:"ams"`
		} `json:"ams"`
	} `json:"print"`
}

func NewMonitor(printers PrinterStore, jobs JobSyncer, notifier EventPublisher) *Monitor {
	return &Monitor{
		printers:      printers,
		jobs:          jobs,
		notifier:      notifier,
		interval:      7 * time.Second,
		known:         make(map[string]uuid.UUID),
		cloudSessions: make(map[string]*cloudSession),
	}
}

func (m *Monitor) Start(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	m.cancel = cancel
	m.client = bambulabs.NewClient(ctx)

	go func() {
		ticker := time.NewTicker(m.interval)
		defer ticker.Stop()
		m.tick(ctx)
		for {
			select {
			case <-ctx.Done():
				_ = m.client.Close()
				m.closeCloudSessions()
				return
			case <-ticker.C:
				m.tick(ctx)
			}
		}
	}()
}

func (m *Monitor) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
}

func (m *Monitor) tick(ctx context.Context) {
	printers, err := m.printers.List(ctx)
	if err != nil {
		log.Printf("bambu monitor list printers: %v", err)
		return
	}

	activeSerials := make(map[string]struct{})
	for _, p := range printers {
		switch {
		case p.HasLANConfig():
			activeSerials[p.LANSerial] = struct{}{}
			if err := m.ensureConnected(p); err != nil {
				log.Printf("bambu lan connect %s (%s): %v", p.Name, p.LANHost, err)
				m.markOffline(ctx, p)
				continue
			}
			if err := m.pollLANPrinter(ctx, p); err != nil {
				log.Printf("bambu lan poll %s: %v", p.Name, err)
			}
		case p.HasCloudConfig():
			if err := m.pollCloudPrinter(ctx, p); err != nil {
				log.Printf("bambu cloud poll %s: %v", p.Name, err)
			}
		}
	}

	m.mu.Lock()
	for serial := range m.known {
		if _, ok := activeSerials[serial]; !ok {
			_ = m.client.Remove(serial)
			delete(m.known, serial)
		}
	}
	m.mu.Unlock()
}

func (m *Monitor) ensureConnected(p printerdomain.Printer) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if id, ok := m.known[p.LANSerial]; ok && id == p.ID {
		if _, err := m.client.Load(p.LANSerial); err == nil {
			return nil
		}
		delete(m.known, p.LANSerial)
	}

	ip := net.ParseIP(strings.TrimSpace(p.LANHost))
	if ip == nil {
		return fmt.Errorf("invalid LAN host %q", p.LANHost)
	}

	_ = m.client.Remove(p.LANSerial)
	_, err := m.client.Add(bambulabs.Config{
		Host:         ip,
		SerialNumber: p.LANSerial,
		AccessCode:   p.LANAccessCode,
		Model:        mapModel(p.Model),
		MQTTPort:     8883,
	})
	if err != nil {
		return err
	}
	m.known[p.LANSerial] = p.ID
	return nil
}

func (m *Monitor) pollLANPrinter(ctx context.Context, p printerdomain.Printer) error {
	printer, err := m.client.Load(p.LANSerial)
	if err != nil {
		return err
	}

	reqCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	_ = printer.RequestUpdate(reqCtx)

	rawState, ok := printer.State()
	if !ok || rawState == nil {
		m.markOffline(ctx, p)
		return nil
	}

	payload, err := json.Marshal(rawState)
	if err != nil {
		return err
	}
	var state printTelemetry
	if err := json.Unmarshal(payload, &state); err != nil {
		return err
	}

	snap, active := snapshotFromTelemetry(state)
	return m.applySnapshot(ctx, p, snap, active)
}

func (m *Monitor) pollCloudPrinter(ctx context.Context, p printerdomain.Printer) error {
	session, err := m.ensureCloudSession(ctx, &p)
	if err != nil {
		m.markOffline(ctx, p)
		return err
	}

	devices, err := session.client.ListDevices()
	if err != nil {
		m.invalidateCloudSession(p)
		m.markOffline(ctx, p)
		return err
	}
	device, ok := findCloudDevice(devices, p.LANSerial)
	if !ok {
		m.markOffline(ctx, p)
		return fmt.Errorf("device %s not bound to cloud account", p.LANSerial)
	}
	if !device.Online {
		m.markOffline(ctx, p)
		return nil
	}

	tasks, _ := session.client.GetTasks(p.LANSerial)
	weightHint := 0
	materialHint := ""
	colorHint := ""
	if tasks != nil {
		restSnap, _ := snapshotFromCloudDevice(device, tasks)
		weightHint = restSnap.EstimatedWeight
		materialHint = restSnap.MaterialHint
		colorHint = restSnap.ColorHint
	}

	if session.pool != nil {
		dataMap, dataErr := session.pool.GetData()
		if dataErr == nil {
			if data, ok := dataMap[device.DevID]; ok && string(data.GcodeState) != "" && string(data.GcodeState) != "UNKNOWN" {
				snap, active := snapshotFromCloudData(data, weightHint, materialHint, colorHint)
				if snap.FileName == "" || snap.ExternalTaskID == "" {
					restSnap, restActive := snapshotFromCloudDevice(device, tasks)
					if snap.FileName == "" {
						snap.FileName = restSnap.FileName
					}
					if snap.ExternalTaskID == "" {
						snap.ExternalTaskID = restSnap.ExternalTaskID
					}
					if !active {
						active = restActive
						snap.Status = restSnap.Status
					}
				}
				return m.applySnapshot(ctx, p, snap, active)
			}
		} else {
			log.Printf("bambu cloud mqtt data %s: %v", p.Name, dataErr)
		}
	}

	snap, active := snapshotFromCloudDevice(device, tasks)
	return m.applySnapshot(ctx, p, snap, active)
}

func (m *Monitor) ensureCloudSession(ctx context.Context, p *printerdomain.Printer) (*cloudSession, error) {
	token := strings.TrimSpace(p.CloudToken)
	if token == "" {
		result, err := CloudLogin(p.CloudEmail, p.CloudPassword, p.CloudRegion)
		if err != nil {
			return nil, err
		}
		if result.NeedsVerify {
			return nil, fmt.Errorf("cloud account requires email verification code")
		}
		token = result.Token
		p.CloudToken = token
		p.UpdatedAt = time.Now()
		_ = m.printers.Update(ctx, *p)
	}

	key := cloudSessionKey(*p, token)
	m.cloudMu.Lock()
	defer m.cloudMu.Unlock()

	if session, ok := m.cloudSessions[key]; ok && session.token == token {
		return session, nil
	}

	client := newCloudClient(token, p.CloudRegion)
	session := &cloudSession{client: client, token: token}
	pool, err := client.GetPrintersAsPool()
	if err == nil {
		if connectErr := pool.ConnectAll(); connectErr != nil {
			log.Printf("bambu cloud mqtt connect: %v", connectErr)
		} else {
			session.pool = pool
		}
	} else {
		log.Printf("bambu cloud pool: %v", err)
	}
	m.cloudSessions[key] = session
	return session, nil
}

func (m *Monitor) invalidateCloudSession(p printerdomain.Printer) {
	m.cloudMu.Lock()
	defer m.cloudMu.Unlock()
	for key, session := range m.cloudSessions {
		if session.token == p.CloudToken || strings.Contains(key, p.CloudEmail) {
			if session.pool != nil {
				session.pool.DisconnectAll()
			}
			delete(m.cloudSessions, key)
		}
	}
}

func (m *Monitor) closeCloudSessions() {
	m.cloudMu.Lock()
	defer m.cloudMu.Unlock()
	for key, session := range m.cloudSessions {
		if session.pool != nil {
			session.pool.DisconnectAll()
		}
		delete(m.cloudSessions, key)
	}
}

func cloudSessionKey(p printerdomain.Printer, token string) string {
	return NormalizeCloudRegion(p.CloudRegion) + "|" + strings.ToLower(strings.TrimSpace(p.CloudEmail)) + "|" + token
}

func (m *Monitor) applySnapshot(ctx context.Context, p printerdomain.Printer, snap printjobusecase.BambuSnapshot, active bool) error {
	printerStatus := mapPrinterStatus(snap.Status, active)
	if p.Status != printerStatus {
		p.Status = printerStatus
		p.UpdatedAt = time.Now()
		_ = m.printers.Update(ctx, p)
	}
	if !active {
		return nil
	}

	job, created, err := m.jobs.SyncFromBambu(ctx, p, snap)
	if err != nil {
		return err
	}
	if m.notifier != nil {
		if created {
			m.notifier.Publish("print_started", "Печать обнаружена на принтере", map[string]any{
				"job_id":     job.ID.String(),
				"printer_id": p.ID.String(),
				"file_name":  job.FileName,
				"is_draft":   job.IsDraft,
			})
		} else if job.Status == printjobdomain.StatusCompleted {
			m.notifier.Publish("print_completed", "Печать завершена", map[string]any{
				"job_id":     job.ID.String(),
				"printer_id": p.ID.String(),
			})
		}
	}
	return nil
}

func (m *Monitor) markOffline(ctx context.Context, p printerdomain.Printer) {
	if p.Status == printerdomain.StatusOffline {
		return
	}
	p.Status = printerdomain.StatusOffline
	p.UpdatedAt = time.Now()
	_ = m.printers.Update(ctx, p)
}

func snapshotFromTelemetry(state printTelemetry) (printjobusecase.BambuSnapshot, bool) {
	print := state.Print
	status, active := mapGcodeState(print.GcodeState)
	fileName := strings.TrimSpace(print.SubtaskName)
	if fileName == "" {
		fileName = strings.TrimSpace(print.GcodeFile)
	}
	fileName = filepath.Base(fileName)

	taskID := strings.TrimSpace(print.TaskID)
	if taskID == "" {
		taskID = strings.TrimSpace(print.SubtaskID)
	}
	if taskID == "" && fileName != "" {
		taskID = fileName + ":" + print.GcodeStartTime
	}

	material, color := trayHints(print)
	return printjobusecase.BambuSnapshot{
		ExternalTaskID: taskID,
		FileName:       fileName,
		Progress:       float64(print.McPercent),
		Status:         status,
		RemainingMin:   print.McRemainingTime,
		MaterialHint:   material,
		ColorHint:      color,
	}, active
}

func trayHints(print struct {
	GcodeFile       string `json:"gcode_file"`
	GcodeStartTime  string `json:"gcode_start_time"`
	GcodeState      string `json:"gcode_state"`
	McPercent       int    `json:"mc_percent"`
	McRemainingTime int    `json:"mc_remaining_time"`
	SubtaskID       string `json:"subtask_id"`
	SubtaskName     string `json:"subtask_name"`
	TaskID          string `json:"task_id"`
	VtTray          struct {
		TrayColor string `json:"tray_color"`
		TrayType  string `json:"tray_type"`
	} `json:"vt_tray"`
	Ams struct {
		TrayNow string `json:"tray_now"`
		Ams     []struct {
			Tray []struct {
				ID        string `json:"id"`
				TrayColor string `json:"tray_color"`
				TrayType  string `json:"tray_type"`
			} `json:"tray"`
		} `json:"ams"`
	} `json:"ams"`
}) (material, color string) {
	material = print.VtTray.TrayType
	color = print.VtTray.TrayColor
	if print.Ams.TrayNow == "" {
		return material, color
	}
	idx, _ := strconv.Atoi(print.Ams.TrayNow)
	for _, unit := range print.Ams.Ams {
		for _, t := range unit.Tray {
			id, _ := strconv.Atoi(t.ID)
			if id == idx {
				if t.TrayType != "" {
					material = t.TrayType
				}
				if t.TrayColor != "" {
					color = t.TrayColor
				}
				return material, color
			}
		}
	}
	return material, color
}

func mapGcodeState(raw string) (printjobdomain.Status, bool) {
	switch bambulabs.GcodeState(strings.ToUpper(strings.TrimSpace(raw))) {
	case bambulabs.RUNNING, bambulabs.PREPARE:
		return printjobdomain.StatusPrinting, true
	case bambulabs.PAUSE:
		return printjobdomain.StatusPaused, true
	case bambulabs.FINISH:
		return printjobdomain.StatusCompleted, true
	case bambulabs.FAILED:
		return printjobdomain.StatusFailed, true
	default:
		return printjobdomain.StatusQueued, false
	}
}

func mapPrinterStatus(status printjobdomain.Status, active bool) printerdomain.PrinterStatus {
	if !active {
		return printerdomain.StatusIdle
	}
	switch status {
	case printjobdomain.StatusPrinting, printjobdomain.StatusQueued, printjobdomain.StatusDraft:
		return printerdomain.StatusPrinting
	case printjobdomain.StatusPaused:
		return printerdomain.StatusPaused
	case printjobdomain.StatusFailed:
		return printerdomain.StatusError
	default:
		return printerdomain.StatusIdle
	}
}

func mapModel(model string) bambulabs.Model {
	switch strings.ToUpper(strings.TrimSpace(model)) {
	case "A1":
		return bambulabs.ModelA1
	case "A1 MINI", "A1MINI", "A1-MINI":
		return bambulabs.ModelA1Mini
	default:
		return bambulabs.ModelA1
	}
}
