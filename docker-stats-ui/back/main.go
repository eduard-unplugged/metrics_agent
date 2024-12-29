package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sync"
	"time"
)

// DockerStats — структура, которую присылают агенты
type DockerStats struct {
	InstanceID  string  `json:"instance_id"`
	ImagesSize  float64 `json:"images_size_gb"`
	Timestamp   string  `json:"timestamp"`
	PruneAction bool    `json:"prune_action"`
}

type statsStorage struct {
	sync.RWMutex
	data map[string]*DockerStats
}

func newStatsStorage() *statsStorage {
	return &statsStorage{
		data: make(map[string]*DockerStats),
	}
}

var storage = newStatsStorage()

// Храним метрики не более суток
const statsRetention = 24 * time.Hour

// htmlTemplate — очень простой шаблон для отображения в браузере
var htmlTemplate = template.Must(template.New("index").Parse(`
<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8" />
  <title>Docker Stats Dashboard</title>
</head>
<body>
  <h1>Docker Stats Dashboard</h1>
  <table border="1" cellpadding="8" cellspacing="0">
    <thead>
      <tr>
        <th>Instance ID</th>
        <th>Images Size (GB)</th>
        <th>Last Update</th>
        <th>Prune Action</th>
        <th>Manual Prune</th>
      </tr>
    </thead>
    <tbody>
      {{range .}}
      <tr>
        <td>{{.InstanceID}}</td>
        <td>{{printf "%.2f" .ImagesSize}}</td>
        <td>{{.Timestamp}}</td>
        <td>{{.PruneAction}}</td>
        <td>
          <form method="POST" action="/api/prune?instance={{.InstanceID}}">
            <button type="submit">Prune</button>
          </form>
        </td>
      </tr>
      {{end}}
    </tbody>
  </table>
</body>
</html>
`))

func main() {
	// Запускаем «очистку старых метрик» каждые полчаса (например)
	go runCleanupTicker()

	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/api/docker-stats", handleDockerStats) // принимает от агентов
	http.HandleFunc("/api/prune", handleManualPrune)        // ручной prune

	addr := ":3000"
	log.Printf("[WEB] Starting on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}

// handleIndex — показывает список инстансов в HTML-таблице
func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET is allowed", http.StatusMethodNotAllowed)
		return
	}
	storage.RLock()
	defer storage.RUnlock()

	var list []*DockerStats
	for _, st := range storage.data {
		list = append(list, st)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := htmlTemplate.Execute(w, list); err != nil {
		log.Printf("Failed to render template: %v", err)
	}
}

// handleDockerStats — агенты присылают сюда (POST /api/docker-stats)
func handleDockerStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST is allowed", http.StatusMethodNotAllowed)
		return
	}
	var stats DockerStats
	if err := json.NewDecoder(r.Body).Decode(&stats); err != nil {
		http.Error(w, "Bad JSON", http.StatusBadRequest)
		return
	}

	// Сохраняем в памяти
	storage.Lock()
	storage.data[stats.InstanceID] = &stats
	storage.Unlock()

	log.Printf("[WEB] Stats from %s: size=%.2fGB, prune=%v, ts=%s",
		stats.InstanceID, stats.ImagesSize, stats.PruneAction, stats.Timestamp)

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("ok\n"))
}

// handleManualPrune — вручную запрашиваем prune на агенте
//
//	POST /api/prune?instance=my-host-123
func handleManualPrune(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST is allowed", http.StatusMethodNotAllowed)
		return
	}
	instanceID := r.URL.Query().Get("instance")
	if instanceID == "" {
		http.Error(w, "instance param required", http.StatusBadRequest)
		return
	}

	log.Printf("[WEB] Manual prune for %s", instanceID)

	// Допустим, агент доступен по http://{instanceID}:8080
	agentURL := fmt.Sprintf("http://%s:8080/prune", instanceID)
	resp, err := http.Post(agentURL, "application/json", nil)
	if err != nil {
		log.Printf("[WEB] Error calling agent: %v", err)
		http.Error(w, "Failed to call agent", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("[WEB] Agent returned %d", resp.StatusCode)
		http.Error(w, "Agent error", http.StatusBadGateway)
		return
	}

	// Если всё ок, удаляем метрику этого инстанса из памяти
	log.Printf("[WEB] Prune success -> removing metrics for %s", instanceID)
	storage.Lock()
	delete(storage.data, instanceID)
	storage.Unlock()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Prune done, metrics deleted\n"))
}

// runCleanupTicker — периодическая очистка данных старше суток
func runCleanupTicker() {
	t := time.NewTicker(30 * time.Minute)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			cleanupOldStats()
		}
	}
}

// cleanupOldStats — удаляет метрики, которым более statsRetention (24 ч)
func cleanupOldStats() {
	now := time.Now()

	storage.Lock()
	defer storage.Unlock()

	for instID, st := range storage.data {
		parsedTime, err := time.Parse(time.RFC3339, st.Timestamp)
		if err != nil {
			log.Printf("[CLEANUP] Can't parse time for %s, removing", instID)
			delete(storage.data, instID)
			continue
		}
		if now.Sub(parsedTime) > statsRetention {
			log.Printf("[CLEANUP] %s is older than 24h, removing", instID)
			delete(storage.data, instID)
		}
	}
}
