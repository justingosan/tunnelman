package models

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

type DockerManager struct {
	client *client.Client
}

// NewDockerManager creates a new Docker manager instance
func NewDockerManager() (*DockerManager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &DockerManager{client: cli}, nil
}

// Close closes the Docker client connection
func (dm *DockerManager) Close() error {
	return dm.client.Close()
}

// IsDockerAvailable checks if Docker is available and running
func (dm *DockerManager) IsDockerAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := dm.client.Ping(ctx)
	return err == nil
}

// findAvailablePort finds a random available port on localhost
func (dm *DockerManager) findAvailablePort() (int, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, fmt.Errorf("failed to find available port: %w", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	return port, nil
}

// StartTraefikContainer starts a Traefik container for the given hostname and returns the assigned port
func (dm *DockerManager) StartTraefikContainer(hostname, originalService, authPassword string) (int, error) {
	ctx := context.Background()
	containerName := GetTraefikContainerName(hostname)

	// Check if container already exists and get its port
	if dm.IsContainerRunning(containerName) {
		port, err := dm.GetContainerPort(containerName)
		if err != nil {
			return 0, fmt.Errorf("failed to get container port: %w", err)
		}
		return port, nil
	}

	// Pull Traefik image first
	if err := dm.PullTraefikImage(); err != nil {
		return 0, fmt.Errorf("failed to pull Traefik image: %w", err)
	}

	// Remove existing container if it exists but is stopped
	dm.RemoveContainer(containerName)

	// Find an available port
	hostPort, err := dm.findAvailablePort()
	if err != nil {
		return 0, fmt.Errorf("failed to find available port: %w", err)
	}

	// Create Traefik config directory
	configDir, err := dm.createTraefikConfig(hostname, originalService, authPassword)
	if err != nil {
		return 0, fmt.Errorf("failed to create Traefik config: %w", err)
	}

	// Create container config
	config := &container.Config{
		Image: "traefik:v3.0",
		Cmd: []string{
			"--providers.file.filename=/etc/traefik/dynamic.yml",
			"--providers.file.watch=true",
			"--entrypoints.web.address=:80",
			"--api.dashboard=false",
			"--log.level=ERROR",
		},
		ExposedPorts: nat.PortSet{
			"80/tcp": struct{}{},
		},
		Labels: map[string]string{
			"tunnelman.hostname": hostname,
			"tunnelman.managed":  "true",
		},
	}

	hostConfig := &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: configDir,
				Target: "/etc/traefik",
			},
		},
		RestartPolicy: container.RestartPolicy{
			Name: "unless-stopped",
		},
		PortBindings: nat.PortMap{
			"80/tcp": []nat.PortBinding{
				{
					HostIP:   "127.0.0.1",
					HostPort: strconv.Itoa(hostPort),
				},
			},
		},
	}

	// Create container
	resp, err := dm.client.ContainerCreate(ctx, config, hostConfig, nil, nil, containerName)
	if err != nil {
		return 0, fmt.Errorf("failed to create Traefik container: %w", err)
	}

	// Start container
	if err := dm.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return 0, fmt.Errorf("failed to start Traefik container: %w", err)
	}

	return hostPort, nil
}

// StopTraefikContainer stops and removes the Traefik container for the given hostname
func (dm *DockerManager) StopTraefikContainer(hostname string) error {
	containerName := GetTraefikContainerName(hostname)

	if !dm.IsContainerRunning(containerName) {
		return nil // Already stopped
	}

	ctx := context.Background()
	timeout := 10

	// Stop container
	if err := dm.client.ContainerStop(ctx, containerName, container.StopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("failed to stop Traefik container: %w", err)
	}

	// Remove container
	if err := dm.RemoveContainer(containerName); err != nil {
		return fmt.Errorf("failed to remove Traefik container: %w", err)
	}

	// Clean up config directory
	if err := dm.removeTraefikConfig(hostname); err != nil {
		return fmt.Errorf("failed to remove Traefik config: %w", err)
	}

	return nil
}

// GetContainerPort gets the host port for a running container
func (dm *DockerManager) GetContainerPort(containerName string) (int, error) {
	ctx := context.Background()
	containerJSON, err := dm.client.ContainerInspect(ctx, containerName)
	if err != nil {
		return 0, fmt.Errorf("failed to inspect container: %w", err)
	}

	// Get port bindings for port 80/tcp
	portBindings := containerJSON.NetworkSettings.Ports["80/tcp"]
	if len(portBindings) == 0 {
		return 0, fmt.Errorf("no port binding found for container %s", containerName)
	}

	hostPort, err := strconv.Atoi(portBindings[0].HostPort)
	if err != nil {
		return 0, fmt.Errorf("failed to parse host port: %w", err)
	}

	return hostPort, nil
}

// IsContainerRunning checks if a container with the given name is running
func (dm *DockerManager) IsContainerRunning(containerName string) bool {
	ctx := context.Background()
	containers, err := dm.client.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return false
	}

	for _, container := range containers {
		for _, name := range container.Names {
			if strings.TrimPrefix(name, "/") == containerName {
				return container.State == "running"
			}
		}
	}

	return false
}

// RemoveContainer removes a container by name
func (dm *DockerManager) RemoveContainer(containerName string) error {
	ctx := context.Background()

	return dm.client.ContainerRemove(ctx, containerName, container.RemoveOptions{
		Force: true,
	})
}

// createTraefikConfig creates the Traefik configuration files
func (dm *DockerManager) createTraefikConfig(hostname, originalService, authPassword string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".tunnelman", "traefik", hostname)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	// Hash the password for basic auth
	hashedPassword, err := HashPassword(authPassword)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}

	// Convert localhost URLs to host.docker.internal so Traefik can reach the host from inside the container
	dockerHostService := originalService
	if strings.Contains(originalService, "localhost") {
		dockerHostService = strings.Replace(originalService, "localhost", "host.docker.internal", 1)
	} else if strings.Contains(originalService, "127.0.0.1") {
		dockerHostService = strings.Replace(originalService, "127.0.0.1", "host.docker.internal", 1)
	}

	// Create dynamic configuration
	dynamicConfig := fmt.Sprintf(`http:
  routers:
    %s:
      rule: "Host(\"%s\")"
      service: %s-service
      middlewares:
        - %s-auth

  services:
    %s-service:
      loadBalancer:
        servers:
          - url: "%s"

  middlewares:
    %s-auth:
      basicAuth:
        users:
          - "tunnelman:%s"
`, hostname, hostname, hostname, hostname, hostname, dockerHostService, hostname, hashedPassword)

	dynamicConfigPath := filepath.Join(configDir, "dynamic.yml")
	if err := os.WriteFile(dynamicConfigPath, []byte(dynamicConfig), 0644); err != nil {
		return "", fmt.Errorf("failed to write dynamic config: %w", err)
	}

	return configDir, nil
}

// removeTraefikConfig removes the Traefik configuration directory
func (dm *DockerManager) removeTraefikConfig(hostname string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".tunnelman", "traefik", hostname)
	return os.RemoveAll(configDir)
}

// PullTraefikImage pulls the Traefik Docker image if not present
func (dm *DockerManager) PullTraefikImage() error {
	ctx := context.Background()

	// Check if image already exists
	if dm.hasTraefikImage() {
		return nil
	}

	reader, err := dm.client.ImagePull(ctx, "traefik:v3.0", image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull Traefik image: %w", err)
	}
	defer reader.Close()

	// Consume the pull output
	_, err = io.Copy(io.Discard, reader)
	return err
}

// hasTraefikImage checks if the Traefik image is already available locally
func (dm *DockerManager) hasTraefikImage() bool {
	ctx := context.Background()
	_, _, err := dm.client.ImageInspectWithRaw(ctx, "traefik:v3.0")
	return err == nil
}
