package v1alpha1

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDeploymentStatusesReportApend(t *testing.T) {
	testStartTime := time.Now()
	testCases := []struct {
		name    string
		current []DeploymentStatus
		append  DeploymentStatus
		want    []DeploymentStatus
	}{{
		name:    "empty",
		current: []DeploymentStatus{},
		append: DeploymentStatus{
			Version: "1", State: HealthStateHealthy, Start: metav1.NewTime(testStartTime),
		},
		want: []DeploymentStatus{
			{Version: "1", State: HealthStateHealthy, Start: metav1.NewTime(testStartTime)},
		},
	}, {
		name: "pending to healthy",
		current: []DeploymentStatus{
			{Version: "1", State: HealthStatePending, Start: metav1.NewTime(testStartTime)},
		},
		append: DeploymentStatus{
			Version: "1", State: HealthStateHealthy, Start: metav1.NewTime(testStartTime.Add(5 * time.Minute)),
		},
		want: []DeploymentStatus{
			{Version: "1", State: HealthStateHealthy, Start: metav1.NewTime(testStartTime.Add(5 * time.Minute))},
		},
	}, {
		name: "new version",
		current: []DeploymentStatus{
			{Version: "1", State: HealthStatePending, Start: metav1.NewTime(testStartTime)},
		},
		append: DeploymentStatus{
			Version: "2", State: HealthStateHealthy, Start: metav1.NewTime(testStartTime.Add(5 * time.Minute)),
		},
		want: []DeploymentStatus{
			{Version: "1", State: HealthStatePending, Start: metav1.NewTime(testStartTime)},
			{Version: "2", State: HealthStateHealthy, Start: metav1.NewTime(testStartTime.Add(5 * time.Minute))},
		},
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			report := DeploymentStatusesReport{Statuses: tc.current}
			report.AppendStatus(tc.append)
			assert.Equal(t, tc.want, report.Statuses)
		})
	}
}
