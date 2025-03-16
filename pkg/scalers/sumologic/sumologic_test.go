package sumologic

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kedacore/keda/v2/pkg/scalers/scalersconfig"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		expectErr bool
	}{
		{
			name: "Valid Config",
			config: &Config{
				Host:      "fake",
				AccessID:  "fake",
				AccessKey: "fake",
			},
		},
		{
			name: "Missing Host",
			config: &Config{
				AccessID:  "fake",
				AccessKey: "fake",
			},
			expectErr: true,
		},
		{
			name: "Missing AccessID",
			config: &Config{
				Host:      "fake",
				AccessKey: "fake",
			},
			expectErr: true,
		},
		{
			name: "Missing AccessKey",
			config: &Config{
				Host:     "fake",
				AccessID: "fake",
			},
			expectErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, err := NewClient(test.config, &scalersconfig.ScalerConfig{})

			if test.expectErr && err != nil {
				return
			}

			if test.expectErr && err == nil {
				t.Error("Expected error, got nil")
			}

			if !test.expectErr && err != nil {
				t.Errorf("Expected no error, got %s", err.Error())
			}

			if !test.expectErr && client == nil {
				t.Error("Expected client to be non-nil")
			}
		})
	}
}

func TestGetLogSearchResult(t *testing.T) {
	tests := []struct {
		name              string
		config            *Config
		query             string
		timerange         time.Duration
		tz                string
		aggregation       string
		expectErr         bool
		createJobResponse LogSearchJobResponse
		jobStatusResponse LogSearchJobStatus
		recordsResponse   LogSearchRecordsResponse
		statusCode        int
		resultField       string
	}{
		{
			name: "Successful Log Search",
			config: &Config{
				Host:      "fake",
				AccessID:  "fake",
				AccessKey: "fake",
				UnsafeSsl: true,
			},
			query:       "test query | count as result",
			timerange:   10,
			tz:          "Asia/Kolkata",
			aggregation: "Latest",
			createJobResponse: LogSearchJobResponse{
				ID: "fake",
			},
			jobStatusResponse: LogSearchJobStatus{
				State:       "DONE GATHERING RESULTS",
				RecordCount: 1,
			},
			recordsResponse: LogSearchRecordsResponse{
				Records: []struct {
					Map map[string]string `json:"map"`
				}{
					{
						Map: map[string]string{"result": "189"},
					},
				},
			},
			statusCode:  http.StatusOK,
			resultField: "result",
		},
		{
			name: "Failed Log Search",
			config: &Config{
				Host:      "fake",
				AccessID:  "fake",
				AccessKey: "fake",
				UnsafeSsl: true,
			},
			query:       "test query",
			timerange:   10,
			tz:          "UTC",
			aggregation: "Latest",
			createJobResponse: LogSearchJobResponse{
				ID: "fake",
			},
			jobStatusResponse: LogSearchJobStatus{
				State:       "CANCELLED",
				RecordCount: 0,
			},
			expectErr:   true,
			statusCode:  http.StatusOK,
			resultField: "result",
		},
		{
			name: "Non-Aggregate Query",
			config: &Config{
				Host:      "fake",
				AccessID:  "fake",
				AccessKey: "fake",
				UnsafeSsl: true,
			},
			query:       "test non-agg query",
			timerange:   10,
			tz:          "UTC",
			aggregation: "Latest",
			createJobResponse: LogSearchJobResponse{
				ID: "fake",
			},
			jobStatusResponse: LogSearchJobStatus{
				State:       "DONE GATHERING RESULTS",
				RecordCount: 0,
			},
			expectErr:   true,
			statusCode:  http.StatusOK,
			resultField: "result",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(test.statusCode)
				w.Header().Set("Content-Type", "application/json")

				if r.Method == "POST" && r.URL.Path == "/api/v1/search/jobs" {
					err := json.NewEncoder(w).Encode(test.createJobResponse)

					if err != nil {
						http.Error(w, fmt.Sprintf("error building the response, %v", err), http.StatusInternalServerError)
						return
					}
				} else if r.Method == "GET" && r.URL.Path == fmt.Sprintf("/api/v1/search/jobs/%s", test.createJobResponse.ID) {
					err := json.NewEncoder(w).Encode(test.jobStatusResponse)

					if err != nil {
						http.Error(w, fmt.Sprintf("error building the response, %v", err), http.StatusInternalServerError)
						return
					}
				} else if r.Method == "GET" && r.URL.Path == fmt.Sprintf("/api/v1/search/jobs/%s/records", test.createJobResponse.ID) {
					err := json.NewEncoder(w).Encode(test.recordsResponse)

					if err != nil {
						http.Error(w, fmt.Sprintf("error building the response, %v", err), http.StatusInternalServerError)
						return
					}
				} else if r.Method == "DELETE" {
					// do nothing
				} else {
					fmt.Println(r.Method, r.URL.Path)
				}
			}))

			defer server.Close()

			test.config.Host = server.URL
			client, err := NewClient(test.config, &scalersconfig.ScalerConfig{
				GlobalHTTPTimeout: 10 * time.Second,
			})
			if err != nil {
				t.Fatalf("Expected no error, got %s", err.Error())
			}

			result, err := client.GetLogSearchResult(test.query, test.timerange, test.aggregation, test.tz, test.resultField)

			if test.expectErr && err != nil {
				return
			}

			if test.expectErr && err == nil {
				t.Error("Expected error, got nil")
			}

			if !test.expectErr && err != nil {
				t.Errorf("Expected no error, got %s", err.Error())
			}

			if !test.expectErr && result == nil {
				t.Error("Expected records to be not nil")
			}
		})
	}
}

func TestGetMetricsSearchResult(t *testing.T) {
	tests := []struct {
		name           string
		config         *Config
		query          string
		quantization   time.Duration
		timerange      time.Duration
		aggregation    string
		tz             string
		expectErr      bool
		response       MetricsQueryResponse
		statusCode     int
		expectedResult float64
		rollup         string
	}{
		{
			name: "Successful Metrics Query - Sum",
			config: &Config{
				Host:      "fake",
				AccessID:  "fake",
				AccessKey: "fake",
				UnsafeSsl: true,
			},
			query:        "test query",
			quantization: 1 * time.Minute,
			timerange:    10 * time.Minute,
			aggregation:  "Sum",
			tz:           "UTC",
			response: MetricsQueryResponse{
				QueryResult: []QueryResult{
					{
						TimeSeriesList: TimeSeriesList{
							TimeSeries: []TimeSeries{
								{
									Points: Points{
										Timestamps: []int64{1, 2, 3},
										Values:     []float64{10, 20, 30},
									},
								},
							},
						},
					},
				},
			},
			statusCode:     http.StatusOK,
			expectedResult: 60,
			rollup:         "Avg",
		},
		{
			name: "Successful Metrics Query - Latest",
			config: &Config{
				Host:      "fake",
				AccessID:  "fake",
				AccessKey: "fake",
				UnsafeSsl: true,
			},
			query:        "test query",
			quantization: 1 * time.Minute,
			timerange:    10 * time.Minute,
			aggregation:  "Latest",
			tz:           "UTC",
			response: MetricsQueryResponse{
				QueryResult: []QueryResult{
					{
						TimeSeriesList: TimeSeriesList{
							TimeSeries: []TimeSeries{
								{
									Points: Points{
										Timestamps: []int64{1, 2, 3},
										Values:     []float64{10, 20, 30},
									},
								},
							},
						},
					},
				},
			},
			statusCode:     http.StatusOK,
			expectedResult: 30,
			rollup:         "Avg",
		},
		{
			name: "Failed Metrics Query",
			config: &Config{
				Host:      "fake",
				AccessID:  "fake",
				AccessKey: "fake",
				UnsafeSsl: true,
			},
			query:        "test query",
			quantization: 1 * time.Minute,
			timerange:    10 * time.Minute,
			aggregation:  "Sum",
			tz:           "UTC",
			expectErr:    true,
			response: MetricsQueryResponse{
				Errors: &QueryErrors{
					Errors: []APIError{
						{
							Code:    "400",
							Message: "Bad Request",
						},
					},
				},
			},
			statusCode: http.StatusBadRequest,
			rollup:     "Avg",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(test.statusCode)
				w.Header().Set("Content-Type", "application/json")

				err := json.NewEncoder(w).Encode(test.response)
				if err != nil {
					http.Error(w, fmt.Sprintf("error building the response, %v", err), http.StatusInternalServerError)
					return
				}
			}))

			defer server.Close()

			test.config.Host = server.URL
			client, err := NewClient(test.config, &scalersconfig.ScalerConfig{
				GlobalHTTPTimeout: 10 * time.Second,
			})
			if err != nil {
				t.Fatalf("Expected no error, got %s", err.Error())
			}

			result, err := client.GetMetricsSearchResult(test.query, test.quantization, test.timerange, test.aggregation, test.tz, test.rollup)

			if test.expectErr && err != nil {
				return
			}

			if test.expectErr && err == nil {
				t.Error("Expected error, got nil")
			}

			if !test.expectErr && err != nil {
				t.Errorf("Expected no error, got %s", err.Error())
			}

			if !test.expectErr && result == nil {
				t.Error("Expected result to be non-nil")
			}

			if !test.expectErr && *result != test.expectedResult {
				t.Errorf("Expected result to be %f, got %f", test.expectedResult, *result)
			}
		})
	}
}
