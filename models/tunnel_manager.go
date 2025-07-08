package models

import (
	"context"
	"fmt"
	"gopkg.in/yaml.v2"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type TunnelManager struct {
	client    *CloudflareClient
	processes map[string]*TunnelProcess
	mutex     sync.RWMutex
	configDir string
}

type TunnelProcess struct {
	PID       int               `json:"pid"`
	TunnelID  string            `json:"tunnel_id"`
	Name      string            `json:"name"`
	Command   []string          `json:"command"`
	StartTime time.Time         `json:"start_time"`
	Status    TunnelStatus      `json:"status"`
	Config    *TunnelConfigFile `json:"config,omitempty"`
	Process   *os.Process       `json:"-"`
}

type TunnelConfigFile struct {
	TunnelID        string                 `yaml:"tunnel"`
	CredentialsFile string                 `yaml:"credentials-file"`
	Ingress         []IngressRule          `yaml:"ingress"`
	LogLevel        string                 `yaml:"loglevel,omitempty"`
	LogFile         string                 `yaml:"logfile,omitempty"`
	Metrics         string                 `yaml:"metrics,omitempty"`
	Protocol        string                 `yaml:"protocol,omitempty"`
	NoAutoUpdate    bool                   `yaml:"no-autoupdate,omitempty"`
	Extra           map[string]interface{} `yaml:",inline"`
}

type IngressRule struct {
	Hostname string                 `yaml:"hostname,omitempty"`
	Service  string                 `yaml:"service"`
	Path     string                 `yaml:"path,omitempty"`
	Headers  map[string]string      `yaml:"headers,omitempty"`
	Extra    map[string]interface{} `yaml:",inline"`
}

func NewTunnelManager(client *CloudflareClient, configDir string) *TunnelManager {
	if configDir == "" {
		configDir = getDefaultConfigDir()
	}

	return &TunnelManager{
		client:    client,
		processes: make(map[string]*TunnelProcess),
		configDir: configDir,
	}
}

func getDefaultConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".cloudflared"
	}
	return filepath.Join(home, ".cloudflared")
}

// Process Management

func (tm *TunnelManager) StartTunnel(ctx context.Context, tunnelName string, config *TunnelConfigFile) (*TunnelProcess, error) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	if process, exists := tm.processes[tunnelName]; exists && process.IsRunning() {
		return nil, fmt.Errorf("tunnel %s is already running with PID %d", tunnelName, process.PID)
	}

	var args []string
	var configPath string

	if config != nil {
		configPath = filepath.Join(tm.configDir, fmt.Sprintf("%s.yml", tunnelName))
		if err := tm.SaveTunnelConfig(tunnelName, config); err != nil {
			return nil, fmt.Errorf("failed to save config: %w", err)
		}
		args = []string{"tunnel", "--config", configPath, "run", tunnelName}
	} else {
		args = []string{"tunnel", "run", tunnelName}
	}

	cmd := exec.CommandContext(ctx, "cloudflared", args...)
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start tunnel: %w", err)
	}

	process := &TunnelProcess{
		PID:       cmd.Process.Pid,
		TunnelID:  tunnelName,
		Name:      tunnelName,
		Command:   append([]string{"cloudflared"}, args...),
		StartTime: time.Now(),
		Status:    StatusActive,
		Config:    config,
		Process:   cmd.Process,
	}

	tm.processes[tunnelName] = process

	go tm.monitorProcess(tunnelName, cmd)

	return process, nil
}

func (tm *TunnelManager) StartTunnelWithURL(ctx context.Context, tunnelName, serviceURL string) (*TunnelProcess, error) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	if process, exists := tm.processes[tunnelName]; exists && process.IsRunning() {
		return nil, fmt.Errorf("tunnel %s is already running with PID %d", tunnelName, process.PID)
	}

	args := []string{"tunnel", "--url", serviceURL, "run", tunnelName}
	cmd := exec.CommandContext(ctx, "cloudflared", args...)
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start tunnel: %w", err)
	}

	process := &TunnelProcess{
		PID:       cmd.Process.Pid,
		TunnelID:  tunnelName,
		Name:      tunnelName,
		Command:   append([]string{"cloudflared"}, args...),
		StartTime: time.Now(),
		Status:    StatusActive,
		Process:   cmd.Process,
	}

	tm.processes[tunnelName] = process

	go tm.monitorProcess(tunnelName, cmd)

	return process, nil
}

func (tm *TunnelManager) StopTunnel(tunnelName string) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	process, exists := tm.processes[tunnelName]
	if !exists {
		return fmt.Errorf("tunnel %s is not running", tunnelName)
	}

	return tm.stopProcess(process)
}

func (tm *TunnelManager) StopTunnelByPID(pid int) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	for name, process := range tm.processes {
		if process.PID == pid {
			if err := tm.stopProcess(process); err != nil {
				return err
			}
			delete(tm.processes, name)
			return nil
		}
	}

	return fmt.Errorf("no tunnel found with PID %d", pid)
}

func (tm *TunnelManager) stopProcess(process *TunnelProcess) error {
	if process.Process == nil {
		return fmt.Errorf("process handle not available")
	}

	err := process.Process.Signal(syscall.SIGTERM)
	if err != nil {
		return fmt.Errorf("failed to send SIGTERM: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := process.Process.Wait()
		done <- err
	}()

	select {
	case <-time.After(10 * time.Second):
		if err := process.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}
		process.Status = StatusInactive
		return nil
	case err := <-done:
		process.Status = StatusInactive
		return err
	}
}

func (tm *TunnelManager) monitorProcess(tunnelName string, cmd *exec.Cmd) {
	err := cmd.Wait()

	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	if process, exists := tm.processes[tunnelName]; exists {
		if err != nil {
			process.Status = StatusError
		} else {
			process.Status = StatusInactive
		}
	}
}

// Status Checking

func (tm *TunnelManager) GetTunnelStatus(tunnelName string) (TunnelStatus, error) {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	process, exists := tm.processes[tunnelName]
	if !exists {
		return StatusInactive, nil
	}

	if !process.IsRunning() {
		process.Status = StatusInactive
		return StatusInactive, nil
	}

	return process.Status, nil
}

func (tm *TunnelManager) IsProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = process.Signal(syscall.Signal(0))
	return err == nil
}

func (tm *TunnelManager) GetRunningTunnels() map[string]*TunnelProcess {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	running := make(map[string]*TunnelProcess)
	for name, process := range tm.processes {
		if process.IsRunning() {
			running[name] = process
		}
	}

	return running
}

func (tm *TunnelManager) GetAllProcesses() map[string]*TunnelProcess {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	processes := make(map[string]*TunnelProcess)
	for name, process := range tm.processes {
		processes[name] = process
	}

	return processes
}

func (tp *TunnelProcess) IsRunning() bool {
	if tp.Process == nil {
		return false
	}

	err := tp.Process.Signal(syscall.Signal(0))
	return err == nil
}

func (tp *TunnelProcess) GetUptime() time.Duration {
	return time.Since(tp.StartTime)
}

func (tp *TunnelProcess) Stop() error {
	if tp.Process == nil {
		return fmt.Errorf("process handle not available")
	}

	err := tp.Process.Signal(syscall.SIGTERM)
	if err != nil {
		return fmt.Errorf("failed to send SIGTERM: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := tp.Process.Wait()
		done <- err
	}()

	select {
	case <-time.After(10 * time.Second):
		if err := tp.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}
		tp.Status = StatusInactive
		return nil
	case err := <-done:
		tp.Status = StatusInactive
		return err
	}
}

// Configuration File Management

func (tm *TunnelManager) SaveTunnelConfig(tunnelName string, config *TunnelConfigFile) error {
	if err := os.MkdirAll(tm.configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath := filepath.Join(tm.configDir, fmt.Sprintf("%s.yml", tunnelName))

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func (tm *TunnelManager) LoadTunnelConfig(tunnelName string) (*TunnelConfigFile, error) {
	configPath := filepath.Join(tm.configDir, fmt.Sprintf("%s.yml", tunnelName))

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", configPath)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config TunnelConfigFile
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

func (tm *TunnelManager) DeleteTunnelConfig(tunnelName string) error {
	configPath := filepath.Join(tm.configDir, fmt.Sprintf("%s.yml", tunnelName))

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil
	}

	if err := os.Remove(configPath); err != nil {
		return fmt.Errorf("failed to delete config file: %w", err)
	}

	return nil
}

func (tm *TunnelManager) ListConfigFiles() ([]string, error) {
	files, err := filepath.Glob(filepath.Join(tm.configDir, "*.yml"))
	if err != nil {
		return nil, fmt.Errorf("failed to list config files: %w", err)
	}

	var configNames []string
	for _, file := range files {
		name := filepath.Base(file)
		name = strings.TrimSuffix(name, ".yml")
		configNames = append(configNames, name)
	}

	return configNames, nil
}

func (tm *TunnelManager) ValidateTunnelConfig(config *TunnelConfigFile) error {
	if config.TunnelID == "" {
		return fmt.Errorf("tunnel ID is required")
	}

	if len(config.Ingress) == 0 {
		return fmt.Errorf("at least one ingress rule is required")
	}

	lastRule := config.Ingress[len(config.Ingress)-1]
	if lastRule.Hostname != "" {
		return fmt.Errorf("last ingress rule must be a catch-all (no hostname)")
	}

	for i, rule := range config.Ingress {
		if rule.Service == "" {
			return fmt.Errorf("service is required for ingress rule %d", i)
		}

		if i < len(config.Ingress)-1 && rule.Hostname == "" {
			return fmt.Errorf("hostname is required for ingress rule %d (except last rule)", i)
		}
	}

	return nil
}

func (tm *TunnelManager) CreateDefaultConfig(tunnelID, tunnelName string) *TunnelConfigFile {
	credentialsFile := filepath.Join(tm.configDir, fmt.Sprintf("%s.json", tunnelID))

	return &TunnelConfigFile{
		TunnelID:        tunnelID,
		CredentialsFile: credentialsFile,
		Ingress: []IngressRule{
			{
				Service: "http_status:404",
			},
		},
		LogLevel: "info",
	}
}

func (tm *TunnelManager) CreateWebServiceConfig(tunnelID, hostname, serviceURL string) *TunnelConfigFile {
	credentialsFile := filepath.Join(tm.configDir, fmt.Sprintf("%s.json", tunnelID))

	return &TunnelConfigFile{
		TunnelID:        tunnelID,
		CredentialsFile: credentialsFile,
		Ingress: []IngressRule{
			{
				Hostname: hostname,
				Service:  serviceURL,
			},
			{
				Service: "http_status:404",
			},
		},
		LogLevel: "info",
	}
}

func (tm *TunnelManager) AddIngressRule(config *TunnelConfigFile, hostname, service string) {
	newRule := IngressRule{
		Hostname: hostname,
		Service:  service,
	}

	if len(config.Ingress) == 0 {
		config.Ingress = []IngressRule{newRule}
		return
	}

	catchAll := config.Ingress[len(config.Ingress)-1]
	config.Ingress = config.Ingress[:len(config.Ingress)-1]
	config.Ingress = append(config.Ingress, newRule, catchAll)
}

func (tm *TunnelManager) RemoveIngressRule(config *TunnelConfigFile, hostname string) bool {
	for i, rule := range config.Ingress {
		if rule.Hostname == hostname {
			config.Ingress = append(config.Ingress[:i], config.Ingress[i+1:]...)
			return true
		}
	}
	return false
}

// Advanced Process Management

func (tm *TunnelManager) RestartTunnel(tunnelName string) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	process, exists := tm.processes[tunnelName]
	if !exists {
		return fmt.Errorf("tunnel %s is not being managed", tunnelName)
	}

	config := process.Config

	if err := tm.stopProcess(process); err != nil {
		return fmt.Errorf("failed to stop tunnel: %w", err)
	}

	delete(tm.processes, tunnelName)

	ctx := context.Background()
	_, err := tm.StartTunnel(ctx, tunnelName, config)
	return err
}

func (tm *TunnelManager) CleanupDeadProcesses() {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	for name, process := range tm.processes {
		if !process.IsRunning() {
			delete(tm.processes, name)
		}
	}
}

func (tm *TunnelManager) StopAllTunnels() error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	var errors []string
	for name, process := range tm.processes {
		if err := tm.stopProcess(process); err != nil {
			errors = append(errors, fmt.Sprintf("failed to stop %s: %v", name, err))
		}
	}

	tm.processes = make(map[string]*TunnelProcess)

	if len(errors) > 0 {
		return fmt.Errorf("errors stopping tunnels: %s", strings.Join(errors, "; "))
	}

	return nil
}

func (tm *TunnelManager) GetProcessByPID(pid int) *TunnelProcess {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	for _, process := range tm.processes {
		if process.PID == pid {
			return process
		}
	}

	return nil
}

func (tm *TunnelManager) KillOrphanedProcesses() error {
	output, err := exec.Command("pgrep", "-f", "cloudflared.*tunnel.*run").CombinedOutput()
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		pid, err := strconv.Atoi(line)
		if err != nil {
			continue
		}

		if tm.GetProcessByPID(pid) == nil {
			if process, err := os.FindProcess(pid); err == nil {
				process.Signal(syscall.SIGTERM)
			}
		}
	}

	return nil
}
