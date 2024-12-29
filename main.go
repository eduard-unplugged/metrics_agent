package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

const (
	// Частота отправки статистики
	statsInterval = time.Hour

	// Частота автоматического prune
	pruneInterval = 24 * time.Hour

	// Порт, на котором слушает агент
	agentPort = ":8080"
)

// DockerStats — структура для отправки на веб-сервер
type DockerStats struct {
	InstanceID  string  `json:"instance_id"`
	ImagesSize  float64 `json:"images_size_gb"`
	Timestamp   string  `json:"timestamp"`
	PruneAction bool    `json:"prune_action"`
}

// AgentConfig — конфиг агента
type AgentConfig struct {
	InstanceID       string
	RemoteServer     string // Куда слать статистику
	DockerAPIVersion string
}

// detectInstanceID — пытается определить ID инстанса (здесь берём hostname)
func detectInstanceID() string {
	host, err := os.Hostname()
	if err != nil {
		return "unknown-host"
	}
	return host
}

// getDockerImagesSizeGB — суммирует размер всех Docker-образов (в ГБ)
func getDockerImagesSizeGB(ctx context.Context, cli *client.Client) (float64, error) {
	images, err := cli.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		return 0, err
	}
	var totalSize int64
	for _, img := range images {
		totalSize += img.Size
	}
	sizeGB := float64(totalSize) / (1024.0 * 1024.0 * 1024.0)
	return sizeGB, nil
}

// sendStats — отправка статистики на веб-сервер (бэкенд)
func sendStats(cfg AgentConfig, stats DockerStats) error {
	data, err := json.Marshal(stats)
	if err != nil {
		return err
	}
	resp, err := http.Post(cfg.RemoteServer, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("non-2xx status code: %d", resp.StatusCode)
	}
	return nil
}

// pruneDocker — удаляет неиспользуемые контейнеры/образы (аналог system prune)
func pruneDocker(ctx context.Context, cli *client.Client) error {
	// Удаляем неиспользуемые контейнеры
	pruneReportC, err := cli.ContainersPrune(ctx, filters.Args{})
	if err != nil {
		return err
	}
	log.Printf("[AGENT] ContainersPrune: %+v", pruneReportC)

	// Удаляем неиспользуемые образы (по умолчанию dangling)
	pruneReportI, err := cli.ImagesPrune(ctx, filters.Args{})
	if err != nil {
		return err
	}
	log.Printf("[AGENT] ImagesPrune: %+v", pruneReportI)
	return nil
}

// doPrune — обёртка для prune + отправка уведомления на сервер
func doPrune(cfg AgentConfig, cli *client.Client) {
	ctx := context.Background()
	if err := pruneDocker(ctx, cli); err != nil {
		log.Printf("[ERROR] Prune failed: %v", err)
		return
	}
	log.Println("[AGENT] Docker prune completed.")

	// Уведомляем веб-сервер
	now := time.Now().Format(time.RFC3339)
	stats := DockerStats{
		InstanceID:  cfg.InstanceID,
		ImagesSize:  0, // Можно заново пересчитать, если нужно
		Timestamp:   now,
		PruneAction: true,
	}
	if err := sendStats(cfg, stats); err != nil {
		log.Printf("[ERROR] Failed to send prune info: %v", err)
	}
}

// startHTTPServer — запускает встраиваемый HTTP-сервер (для ручного prune)
func startHTTPServer(cfg AgentConfig, cli *client.Client) {
	http.HandleFunc("/prune", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
			return
		}
		go doPrune(cfg, cli)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Prune initiated.\n"))
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK\n"))
	})

	log.Printf("[AGENT] Listening on %s", agentPort)
	if err := http.ListenAndServe(agentPort, nil); err != nil {
		log.Fatalf("[FATAL] Agent HTTP server failed: %v", err)
	}
}

func main() {
	instanceID := detectInstanceID()

	remoteServer := os.Getenv("REMOTE_SERVER_URL")
	if remoteServer == "" {
		remoteServer = "http://localhost:3000/api/docker-stats"
	}

	dockerAPIVersion := os.Getenv("DOCKER_API_VERSION")
	if dockerAPIVersion == "" {
		dockerAPIVersion = "1.41"
	}

	cfg := AgentConfig{
		InstanceID:       instanceID,
		RemoteServer:     remoteServer,
		DockerAPIVersion: dockerAPIVersion,
	}

	cli, err := client.NewClientWithOpts(client.WithVersion(cfg.DockerAPIVersion))
	if err != nil {
		log.Fatalf("[FATAL] Failed to create Docker client: %v", err)
	}

	ctx := context.Background()

	// Тикер для отправки статистики раз в час
	statsTicker := time.NewTicker(statsInterval)
	defer statsTicker.Stop()

	// Тикер для автоматического prune раз в день
	pruneTicker := time.NewTicker(pruneInterval)
	defer pruneTicker.Stop()

	go startHTTPServer(cfg, cli)

	log.Printf("[AGENT] Started. InstanceID=%s. RemoteServer=%s", cfg.InstanceID, cfg.RemoteServer)

	for {
		select {
		case <-statsTicker.C:
			// Сбор и отправка статистики
			sizeGB, err := getDockerImagesSizeGB(ctx, cli)
			if err != nil {
				log.Printf("[ERROR] getDockerImagesSizeGB: %v", err)
				continue
			}
			now := time.Now().Format(time.RFC3339)
			stats := DockerStats{
				InstanceID:  cfg.InstanceID,
				ImagesSize:  sizeGB,
				Timestamp:   now,
				PruneAction: false,
			}
			if err := sendStats(cfg, stats); err != nil {
				log.Printf("[ERROR] sendStats: %v", err)
			}

		case <-pruneTicker.C:
			// Автоприн раз в день
			log.Println("[AGENT] Daily prune triggered")
			doPrune(cfg, cli)
		}
	}
}
