package web

import (
	"encoding/json"
	"net/http"
)

type prometheusTargetGroup struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels"`
}

func (h *handler) handlePrometheusTargets(writer http.ResponseWriter, r *http.Request) {
	endpoints, err := h.endpoints.List(r.Context())
	if err != nil {
		h.logger.Error("failed to list endpoints", "error", err)

		http.Error(writer, "failed to list endpoints", http.StatusInternalServerError)
		return
	}

	targets := make([]prometheusTargetGroup, 0, len(endpoints))

	for _, endpoint := range endpoints {
		targets = append(targets, prometheusTargetGroup{
			Targets: []string{endpoint.URL},
			Labels: map[string]string{
				"signaldock_id": endpoint.ID.String(),
			},
		},
		)
	}

	payload, err := json.Marshal(targets)
	if err != nil {
		h.logger.Error("failed to encode Prometheus targets", "error", err)
		http.Error(writer, "failed to encode targets", http.StatusInternalServerError)
		return
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)

	_, _ = writer.Write(payload)
}
