//go:build e2e
// +build e2e

package operator_prometheus_metrics_test

import (
	"fmt"
	"strings"
	"testing"

	prommodel "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/stretchr/testify/assert"

	. "github.com/kedacore/http-add-on/tests/helper"
)

const (
	testName = "operator-prom-metrics-test"
)

var (
	testNamespace             = fmt.Sprintf("%s-ns", testName)
	clientName                = fmt.Sprintf("%s-client", testName)
	kedaOperatorPrometheusURL = "http://keda-add-ons-http-operator-metrics.keda:8080/metrics"
)

type templateData struct {
	TestNamespace string
	ClientName    string
}

const (
	clientTemplate = `
apiVersion: v1
kind: Pod
metadata:
  name: {{.ClientName}}
  namespace: {{.TestNamespace}}
spec:
  containers:
  - name: {{.ClientName}}
    image: curlimages/curl
    command:
      - sh
      - -c
      - "exec tail -f /dev/null"`
)

func TestOperatorMetricsEndpoint(t *testing.T) {
	// setup
	t.Log("--- setting up ---")
	// Create kubernetes resources
	kc := GetKubernetesClient(t)
	data, templates := getTemplateData()
	CreateKubernetesResources(t, kc, testNamespace, data, templates)

	// Wait for all pods to be running
	assert.True(t, WaitForAllPodRunningInNamespace(t, kc, testNamespace, 60, 1),
		"pods in namespace should be running")

	// Fetch metrics and validate they are accessible
	family := fetchAndParsePrometheusMetrics(t, fmt.Sprintf("curl %s", kedaOperatorPrometheusURL))
	
	// Verify that we can get metrics (at least some metrics should be present from controller-runtime)
	assert.NotEmpty(t, family, "operator metrics endpoint should return metrics")
	
	// Check for common controller-runtime metrics
	_, hasWorkqueueMetric := family["workqueue_depth"]
	_, hasGoMetric := family["go_info"]
	
	assert.True(t, hasWorkqueueMetric || hasGoMetric, 
		"operator should expose standard controller-runtime or Go metrics")

	// cleanup
	DeleteKubernetesResources(t, testNamespace, data, templates)
}

func fetchAndParsePrometheusMetrics(t *testing.T, cmd string) map[string]*prommodel.MetricFamily {
	out, _, err := ExecCommandOnSpecificPod(t, clientName, testNamespace, cmd)
	assert.NoErrorf(t, err, "cannot execute command - %s", err)

	parser := expfmt.TextParser{}
	// Ensure EOL
	reader := strings.NewReader(strings.ReplaceAll(out, "\r\n", "\n"))
	families, err := parser.TextToMetricFamilies(reader)
	assert.NoErrorf(t, err, "cannot parse metrics - %s", err)

	return families
}

func getTemplateData() (templateData, []Template) {
	return templateData{
			TestNamespace: testNamespace,
			ClientName:    clientName,
		}, []Template{
			{Name: "clientTemplate", Config: clientTemplate},
		}
}
