package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ----------------------------------------------------------------------------
// Métricas Prometheus (requisitos obrigatórios da Parte 2)
// ----------------------------------------------------------------------------

var (
	// httpRequestsTotal expõe o VOLUME DE REQUISIÇÕES, segmentado por
	// método HTTP, rota e código de status.
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Número total de requisições HTTP processadas pelo serviço.",
		},
		[]string{"method", "path", "status"},
	)

	// httpRequestDuration registra a latência das requisições (histograma),
	// útil para análise de comportamento no dashboard do Grafana.
	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duração das requisições HTTP em segundos.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"path"},
	)
)

// ----------------------------------------------------------------------------
// Tipos e middleware
// ----------------------------------------------------------------------------

// ProjetoResponse define o contrato JSON retornado pelo endpoint /projeto-korp.
type ProjetoResponse struct {
	Nome    string `json:"nome"`
	Horario string `json:"horario"`
}

// statusRecorder envolve o http.ResponseWriter para capturar o código de
// status HTTP efetivamente escrito na resposta.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// instrument é um middleware que mede a duração e contabiliza cada requisição
// nas métricas do Prometheus.
func instrument(path string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		inicio := time.Now()

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next(rec, r)

		duracao := time.Since(inicio).Seconds()
		httpRequestDuration.WithLabelValues(path).Observe(duracao)
		httpRequestsTotal.WithLabelValues(r.Method, path, strconv.Itoa(rec.status)).Inc()
	}
}

// ----------------------------------------------------------------------------
// Handlers
// ----------------------------------------------------------------------------

// projetoKorpHandler responde GET /projeto-korp com o nome do projeto e o
// horário atual em UTC (formato RFC3339), resolvido a cada requisição.
func projetoKorpHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	resposta := ProjetoResponse{
		Nome:    "Projeto Korp",
		Horario: time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(resposta); err != nil {
		http.Error(w, "Erro ao serializar resposta", http.StatusInternalServerError)
		return
	}
}

// healthHandler expõe a DISPONIBILIDADE do serviço através de um endpoint
// dedicado (GET /health). Complementa a métrica "up" gerada pelo Prometheus.
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// ----------------------------------------------------------------------------
// main
// ----------------------------------------------------------------------------

func main() {
	mux := http.NewServeMux()

	// Endpoints da aplicação (instrumentados).
	mux.HandleFunc("/projeto-korp", instrument("/projeto-korp", projetoKorpHandler))
	mux.HandleFunc("/health", instrument("/health", healthHandler))

	// Endpoint de métricas oficial do Prometheus (não instrumentado para
	// evitar ruído nas próprias métricas de scrape).
	mux.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Println("Servidor HTTP iniciado na porta 8080")
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Falha ao iniciar o servidor: %v", err)
	}
}
