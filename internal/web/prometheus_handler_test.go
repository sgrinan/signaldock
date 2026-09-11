package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"uuid"

	"github.com/sgrinan/signaldock/internal/endpoint"
)

func TestHandler_HandlePrometheusTargets(t *testing.T) {
	tests := []struct {
		name      string
		endpoints []endpoint.Endpoint
		want      []prometheusTargetGroup
	}{
		{
			name:      "empty",
			endpoints: nil,
			want:      []prometheusTargetGroup{},
		},
		{
			name: "endpoints",
			endpoints: []endpoint.Endpoint{
				{
					ID:  uuid.NewV7(),
					URL: "https://example.com/",
				},
				{
					ID:  uuid.NewV7(),
					URL: "https://www.google.es/",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.want == nil {
				tt.want = make([]prometheusTargetGroup, 0, len(tt.endpoints))

				for _, item := range tt.endpoints {
					tt.want = append(tt.want, prometheusTargetGroup{
						Targets: []string{item.URL},
						Labels: map[string]string{
							"signaldock_id": item.ID.String(),
						},
					},
					)
				}
			}

			service := &fakeEndpointService{
				listFunc: func() ([]endpoint.Endpoint, error) {
					return tt.endpoints, nil
				},
			}

			handler := newTestHandler(service)

			request := httptest.NewRequest(http.MethodGet, "/api/prometheus/targets", nil)
			recorder := httptest.NewRecorder()

			handler.handlePrometheusTargets(recorder, request)

			if got, want := recorder.Code, http.StatusOK; got != want {
				t.Errorf("handlePrometheusTargets() status = %d, want %d", got, want)
			}

			if got, want := recorder.Header().Get("Content-Type"), "application/json"; got != want {
				t.Errorf("handlePrometheusTargets() Content-Type = %q, want %q", got, want)
			}

			var got []prometheusTargetGroup

			if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
				t.Fatalf("json.Unmarshal() error = %v, want nil", err)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("handlePrometheusTargets() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
