package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestWebhookHandler tests the webhookHandler function.
func TestWebhookHandler(t *testing.T) {
	testCases := []struct {
		description      string
		request          string
		expectedStatus   int
		expectedResponse string
	}{
		// Test DaemonSets
		{
			description:      "CREATE DaemonSet without toleration",
			request:          makeAdmissionRequest("DaemonSet", "CREATE", "foo/test-ds", ""),
			expectedStatus:   http.StatusOK,
			expectedResponse: `{"response":{"uid":"f0b23c24-35f6-42a3-99e3-aa4ccab85f91","allowed":true,"patch":"W3sib3AiOiJyZXBsYWNlIiwicGF0aCI6Ii9zcGVjL3RlbXBsYXRlL3NwZWMvdG9sZXJhdGlvbnMiLCJ2YWx1ZSI6W3sia2V5IjoiU2ltdWxhdGVOb2RlRmFpbHVyZSIsIm9wZXJhdG9yIjoiRXhpc3RzIiwiZWZmZWN0IjoiTm9FeGVjdXRlIn1dfSx7Im9wIjoicmVwbGFjZSIsInBhdGgiOiIvbWV0YWRhdGEvYW5ub3RhdGlvbnMiLCJ2YWx1ZSI6eyJzb21lX2Fubm90YXRpb24iOiJzb21lX3ZhbHVlIiwidXBkYXRlZF9ieSI6InRvbGVyYXRpb25XZWJob29rIn19XQ==","warnings":["DaemonSet foo/test-ds does not have a toleration set.","DaemonSet foo/test-ds was updated with toleration."]}}`,
		},
		{
			description:      "CREATE DaemonSet with toleration set to other toleration",
			request:          makeAdmissionRequest("DaemonSet", "CREATE", "foo/test-ds", "TestToleration"),
			expectedStatus:   http.StatusOK,
			expectedResponse: `{"response":{"uid":"f0b23c24-35f6-42a3-99e3-aa4ccab85f91","allowed":true,"patch":"W3sib3AiOiJyZXBsYWNlIiwicGF0aCI6Ii9zcGVjL3RlbXBsYXRlL3NwZWMvdG9sZXJhdGlvbnMiLCJ2YWx1ZSI6W3sia2V5IjoiVGVzdFRvbGVyYXRpb24iLCJvcGVyYXRvciI6IkV4aXN0cyIsImVmZmVjdCI6Ik5vRXhlY3V0ZSJ9LHsia2V5IjoiU2ltdWxhdGVOb2RlRmFpbHVyZSIsIm9wZXJhdG9yIjoiRXhpc3RzIiwiZWZmZWN0IjoiTm9FeGVjdXRlIn1dfSx7Im9wIjoicmVwbGFjZSIsInBhdGgiOiIvbWV0YWRhdGEvYW5ub3RhdGlvbnMiLCJ2YWx1ZSI6eyJzb21lX2Fubm90YXRpb24iOiJzb21lX3ZhbHVlIiwidXBkYXRlZF9ieSI6InRvbGVyYXRpb25XZWJob29rIn19XQ==","warnings":["DaemonSet foo/test-ds does not have a toleration set.","DaemonSet foo/test-ds was updated with toleration."]}}`,
		},
		{
			description:      "CREATE DaemonSet with toleration set to target toleration",
			request:          makeAdmissionRequest("DaemonSet", "CREATE", "foo/test-ds", "SimulateNodeFailure"),
			expectedStatus:   http.StatusOK,
			expectedResponse: `{"response":{"uid":"f0b23c24-35f6-42a3-99e3-aa4ccab85f91","allowed":true}}`,
		},
		{
			description:      "UPDATE DaemonSet without toleration",
			request:          makeAdmissionRequest("DaemonSet", "UPDATE", "foo/test-ds", ""),
			expectedStatus:   http.StatusOK,
			expectedResponse: `{"response":{"uid":"f0b23c24-35f6-42a3-99e3-aa4ccab85f91","allowed":true,"patch":"W3sib3AiOiJyZXBsYWNlIiwicGF0aCI6Ii9zcGVjL3RlbXBsYXRlL3NwZWMvdG9sZXJhdGlvbnMiLCJ2YWx1ZSI6W3sia2V5IjoiU2ltdWxhdGVOb2RlRmFpbHVyZSIsIm9wZXJhdG9yIjoiRXhpc3RzIiwiZWZmZWN0IjoiTm9FeGVjdXRlIn1dfSx7Im9wIjoicmVwbGFjZSIsInBhdGgiOiIvbWV0YWRhdGEvYW5ub3RhdGlvbnMiLCJ2YWx1ZSI6eyJzb21lX2Fubm90YXRpb24iOiJzb21lX3ZhbHVlIiwidXBkYXRlZF9ieSI6InRvbGVyYXRpb25XZWJob29rIn19XQ==","warnings":["DaemonSet foo/test-ds does not have a toleration set.","DaemonSet foo/test-ds was updated with toleration."]}}`,
		},
		{
			description:      "UPDATE DaemonSet with toleration set to other toleration",
			request:          makeAdmissionRequest("DaemonSet", "UPDATE", "foo/test-ds", "TestToleration"),
			expectedStatus:   http.StatusOK,
			expectedResponse: `{"response":{"uid":"f0b23c24-35f6-42a3-99e3-aa4ccab85f91","allowed":true,"patch":"W3sib3AiOiJyZXBsYWNlIiwicGF0aCI6Ii9zcGVjL3RlbXBsYXRlL3NwZWMvdG9sZXJhdGlvbnMiLCJ2YWx1ZSI6W3sia2V5IjoiVGVzdFRvbGVyYXRpb24iLCJvcGVyYXRvciI6IkV4aXN0cyIsImVmZmVjdCI6Ik5vRXhlY3V0ZSJ9LHsia2V5IjoiU2ltdWxhdGVOb2RlRmFpbHVyZSIsIm9wZXJhdG9yIjoiRXhpc3RzIiwiZWZmZWN0IjoiTm9FeGVjdXRlIn1dfSx7Im9wIjoicmVwbGFjZSIsInBhdGgiOiIvbWV0YWRhdGEvYW5ub3RhdGlvbnMiLCJ2YWx1ZSI6eyJzb21lX2Fubm90YXRpb24iOiJzb21lX3ZhbHVlIiwidXBkYXRlZF9ieSI6InRvbGVyYXRpb25XZWJob29rIn19XQ==","warnings":["DaemonSet foo/test-ds does not have a toleration set.","DaemonSet foo/test-ds was updated with toleration."]}}`,
		},
		{
			description:      "UPDATE DaemonSet with toleration set to target toleration",
			request:          makeAdmissionRequest("DaemonSet", "UPDATE", "foo/test-ds", "SimulateNodeFailure"),
			expectedStatus:   http.StatusOK,
			expectedResponse: `{"response":{"uid":"f0b23c24-35f6-42a3-99e3-aa4ccab85f91","allowed":true}}`,
		},
		// Test Deployments
		{
			description:      "CREATE Deployment without toleration",
			request:          makeAdmissionRequest("Deployment", "CREATE", "foo/test-dep", ""),
			expectedStatus:   http.StatusOK,
			expectedResponse: `{"response":{"uid":"f0b23c24-35f6-42a3-99e3-aa4ccab85f91","allowed":true,"patch":"W3sib3AiOiJyZXBsYWNlIiwicGF0aCI6Ii9zcGVjL3RlbXBsYXRlL3NwZWMvdG9sZXJhdGlvbnMiLCJ2YWx1ZSI6W3sia2V5IjoiU2ltdWxhdGVOb2RlRmFpbHVyZSIsIm9wZXJhdG9yIjoiRXhpc3RzIiwiZWZmZWN0IjoiTm9FeGVjdXRlIn1dfSx7Im9wIjoicmVwbGFjZSIsInBhdGgiOiIvbWV0YWRhdGEvYW5ub3RhdGlvbnMiLCJ2YWx1ZSI6eyJzb21lX2Fubm90YXRpb24iOiJzb21lX3ZhbHVlIiwidXBkYXRlZF9ieSI6InRvbGVyYXRpb25XZWJob29rIn19XQ==","warnings":["Deployment foo/test-dep does not have a toleration set.","Deployment foo/test-dep was updated with toleration."]}}`,
		},
		{
			description:      "CREATE Deployment with toleration set to other toleration",
			request:          makeAdmissionRequest("Deployment", "CREATE", "foo/test-dep", "TestToleration"),
			expectedStatus:   http.StatusOK,
			expectedResponse: `{"response":{"uid":"f0b23c24-35f6-42a3-99e3-aa4ccab85f91","allowed":true,"patch":"W3sib3AiOiJyZXBsYWNlIiwicGF0aCI6Ii9zcGVjL3RlbXBsYXRlL3NwZWMvdG9sZXJhdGlvbnMiLCJ2YWx1ZSI6W3sia2V5IjoiVGVzdFRvbGVyYXRpb24iLCJvcGVyYXRvciI6IkV4aXN0cyIsImVmZmVjdCI6Ik5vRXhlY3V0ZSJ9LHsia2V5IjoiU2ltdWxhdGVOb2RlRmFpbHVyZSIsIm9wZXJhdG9yIjoiRXhpc3RzIiwiZWZmZWN0IjoiTm9FeGVjdXRlIn1dfSx7Im9wIjoicmVwbGFjZSIsInBhdGgiOiIvbWV0YWRhdGEvYW5ub3RhdGlvbnMiLCJ2YWx1ZSI6eyJzb21lX2Fubm90YXRpb24iOiJzb21lX3ZhbHVlIiwidXBkYXRlZF9ieSI6InRvbGVyYXRpb25XZWJob29rIn19XQ==","warnings":["Deployment foo/test-dep does not have a toleration set.","Deployment foo/test-dep was updated with toleration."]}}`,
		},
		{
			description:      "CREATE Deployment with toleration set to target toleration",
			request:          makeAdmissionRequest("Deployment", "CREATE", "foo/test-dep", "SimulateNodeFailure"),
			expectedStatus:   http.StatusOK,
			expectedResponse: `{"response":{"uid":"f0b23c24-35f6-42a3-99e3-aa4ccab85f91","allowed":true}}`,
		},
		{
			description:      "UPDATE Deployment without toleration",
			request:          makeAdmissionRequest("Deployment", "UPDATE", "foo/test-dep", ""),
			expectedStatus:   http.StatusOK,
			expectedResponse: `{"response":{"uid":"f0b23c24-35f6-42a3-99e3-aa4ccab85f91","allowed":true,"patch":"W3sib3AiOiJyZXBsYWNlIiwicGF0aCI6Ii9zcGVjL3RlbXBsYXRlL3NwZWMvdG9sZXJhdGlvbnMiLCJ2YWx1ZSI6W3sia2V5IjoiU2ltdWxhdGVOb2RlRmFpbHVyZSIsIm9wZXJhdG9yIjoiRXhpc3RzIiwiZWZmZWN0IjoiTm9FeGVjdXRlIn1dfSx7Im9wIjoicmVwbGFjZSIsInBhdGgiOiIvbWV0YWRhdGEvYW5ub3RhdGlvbnMiLCJ2YWx1ZSI6eyJzb21lX2Fubm90YXRpb24iOiJzb21lX3ZhbHVlIiwidXBkYXRlZF9ieSI6InRvbGVyYXRpb25XZWJob29rIn19XQ==","warnings":["Deployment foo/test-dep does not have a toleration set.","Deployment foo/test-dep was updated with toleration."]}}`,
		},
		{
			description:      "UPDATE Deployment with toleration set to other toleration",
			request:          makeAdmissionRequest("Deployment", "UPDATE", "foo/test-dep", "TestToleration"),
			expectedStatus:   http.StatusOK,
			expectedResponse: `{"response":{"uid":"f0b23c24-35f6-42a3-99e3-aa4ccab85f91","allowed":true,"patch":"W3sib3AiOiJyZXBsYWNlIiwicGF0aCI6Ii9zcGVjL3RlbXBsYXRlL3NwZWMvdG9sZXJhdGlvbnMiLCJ2YWx1ZSI6W3sia2V5IjoiVGVzdFRvbGVyYXRpb24iLCJvcGVyYXRvciI6IkV4aXN0cyIsImVmZmVjdCI6Ik5vRXhlY3V0ZSJ9LHsia2V5IjoiU2ltdWxhdGVOb2RlRmFpbHVyZSIsIm9wZXJhdG9yIjoiRXhpc3RzIiwiZWZmZWN0IjoiTm9FeGVjdXRlIn1dfSx7Im9wIjoicmVwbGFjZSIsInBhdGgiOiIvbWV0YWRhdGEvYW5ub3RhdGlvbnMiLCJ2YWx1ZSI6eyJzb21lX2Fubm90YXRpb24iOiJzb21lX3ZhbHVlIiwidXBkYXRlZF9ieSI6InRvbGVyYXRpb25XZWJob29rIn19XQ==","warnings":["Deployment foo/test-dep does not have a toleration set.","Deployment foo/test-dep was updated with toleration."]}}`,
		},
		{
			description:      "UPDATE Deployment with toleration set to target toleration",
			request:          makeAdmissionRequest("Deployment", "UPDATE", "foo/test-dep", "SimulateNodeFailure"),
			expectedStatus:   http.StatusOK,
			expectedResponse: `{"response":{"uid":"f0b23c24-35f6-42a3-99e3-aa4ccab85f91","allowed":true}}`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			req := bytes.NewBufferString(testCase.request)

			server := httptest.NewServer(http.HandlerFunc(webhookHandler))
			defer server.Close()
			resp, err := http.Post(server.URL, jsonContentType, req)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != testCase.expectedStatus {
				t.Errorf("Expected status code %d, got %d", testCase.expectedStatus, resp.StatusCode)
			}
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != testCase.expectedResponse {
				t.Errorf("Expected response body %s, got %s", testCase.expectedResponse, string(data))
			}
		})
	}
}

// makeAdmissionRequest is a helper function to create an AdmissionReview request
func makeAdmissionRequest(k8sObjectKind, k8sApiEvent, k8sObjectFullName, tolerationKey string) string {
	k8sObjectNamespace, k8sObjectName := strings.Split(k8sObjectFullName, "/")[0], strings.Split(k8sObjectFullName, "/")[1]
	k8sObect := fmt.Sprintf(
		`{
			"kind": "AdmissionReview",
			"apiVersion": "admission.k8s.io/v1beta1",
			"request": {
			  "uid": "f0b23c24-35f6-42a3-99e3-aa4ccab85f91",
			  "kind": {
				"group": "apps",
				"version": "v1",
				"kind": "%s"
			  },
			  "operation": "%s",
			  "userInfo": {
				"username": "someuser@gmail.com"
			  },
			  "object": {
				"kind": "%s",
				"apiVersion": "apps/v1",
				"metadata": {
				  "name": "%s",
				  "namespace": "%s",
				  "annotations": {
					"some_annotation": "some_value"
				  }
				},
				%s
			  }
			}
		  }`,
		k8sObjectKind,
		k8sApiEvent,
		k8sObjectKind,
		k8sObjectName,
		k8sObjectNamespace,
		getTolerationPodSpec(tolerationKey),
	)
	return k8sObect
}

// getTolerationPodSpec is a helper function to create a pod spec with a toleration
func getTolerationPodSpec(tolerationKey string) string {
	if tolerationKey == "" {
		return `"spec": {"template": {"spec": {"restartPolicy": "Always"}}}`
	} else {
		return fmt.Sprintf(
			`"spec": {"template": {"spec": {"restartPolicy": "Always", "tolerations": [{"key": "%s", "operator": "Exists", "effect": "NoExecute"}]}}}`,
			tolerationKey,
		)
	}
}
