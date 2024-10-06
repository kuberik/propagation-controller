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
		name: "healthy to healthy",
		current: []DeploymentStatus{
			{Version: "1", State: HealthStateHealthy, Start: metav1.NewTime(testStartTime)},
		},
		append: DeploymentStatus{
			Version: "1", State: HealthStateHealthy, Start: metav1.NewTime(testStartTime.Add(5 * time.Minute)),
		},
		want: []DeploymentStatus{
			{Version: "1", State: HealthStateHealthy, Start: metav1.NewTime(testStartTime)},
		},
	}, {
		name: "unhealthy to unhealthy",
		current: []DeploymentStatus{
			{Version: "1", State: HealthStateUnhealthy, Start: metav1.NewTime(testStartTime)},
		},
		append: DeploymentStatus{
			Version: "1", State: HealthStateUnhealthy, Start: metav1.NewTime(testStartTime.Add(5 * time.Minute)),
		},
		want: []DeploymentStatus{
			{Version: "1", State: HealthStateUnhealthy, Start: metav1.NewTime(testStartTime)},
		},
	}, {
		name: "healthy to unhealthy",
		current: []DeploymentStatus{
			{Version: "1", State: HealthStateHealthy, Start: metav1.NewTime(testStartTime)},
		},
		append: DeploymentStatus{
			Version: "1", State: HealthStateUnhealthy, Start: metav1.NewTime(testStartTime.Add(5 * time.Minute)),
		},
		want: []DeploymentStatus{
			{Version: "1", State: HealthStateHealthy, Start: metav1.NewTime(testStartTime)},
			{Version: "1", State: HealthStateUnhealthy, Start: metav1.NewTime(testStartTime.Add(5 * time.Minute))},
		},
	}, {
		name: "unhealthy to healthy",
		current: []DeploymentStatus{
			{Version: "1", State: HealthStateUnhealthy, Start: metav1.NewTime(testStartTime)},
		},
		append: DeploymentStatus{
			Version: "1", State: HealthStateHealthy, Start: metav1.NewTime(testStartTime.Add(5 * time.Minute)),
		},
		want: []DeploymentStatus{
			{Version: "1", State: HealthStateUnhealthy, Start: metav1.NewTime(testStartTime)},
			{Version: "1", State: HealthStateHealthy, Start: metav1.NewTime(testStartTime.Add(5 * time.Minute))},
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
		name: "pending to unhealthy",
		current: []DeploymentStatus{
			{Version: "1", State: HealthStatePending, Start: metav1.NewTime(testStartTime)},
		},
		append: DeploymentStatus{
			Version: "1", State: HealthStateUnhealthy, Start: metav1.NewTime(testStartTime.Add(5 * time.Minute)),
		},
		want: []DeploymentStatus{
			{Version: "1", State: HealthStateUnhealthy, Start: metav1.NewTime(testStartTime)},
		},
	}, {
		name: "pending to pending",
		current: []DeploymentStatus{
			{Version: "1", State: HealthStatePending, Start: metav1.NewTime(testStartTime)},
		},
		append: DeploymentStatus{
			Version: "1", State: HealthStatePending, Start: metav1.NewTime(testStartTime.Add(5 * time.Minute)),
		},
		want: []DeploymentStatus{
			{Version: "1", State: HealthStatePending, Start: metav1.NewTime(testStartTime)},
		},
	}, {
		name: "pending to unhealthy",
		current: []DeploymentStatus{
			{Version: "1", State: HealthStatePending, Start: metav1.NewTime(testStartTime)},
		},
		append: DeploymentStatus{
			Version: "1", State: HealthStateUnhealthy, Start: metav1.NewTime(testStartTime.Add(5 * time.Minute)),
		},
		want: []DeploymentStatus{
			{Version: "1", State: HealthStateUnhealthy, Start: metav1.NewTime(testStartTime)},
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
	}, {
		name: "pending again for the same version",
		current: []DeploymentStatus{
			{Version: "1", State: HealthStateHealthy, Start: metav1.NewTime(testStartTime)},
		},
		append: DeploymentStatus{
			Version: "1", State: HealthStatePending, Start: metav1.NewTime(testStartTime.Add(5 * time.Minute)),
		},
		want: []DeploymentStatus{
			{Version: "1", State: HealthStateHealthy, Start: metav1.NewTime(testStartTime)},
			{Version: "1", State: HealthStatePending, Start: metav1.NewTime(testStartTime.Add(5 * time.Minute))},
		},
	}, {
		name: "compact to healthy after pending gets healthy for the same version",
		current: []DeploymentStatus{
			{Version: "1", State: HealthStateHealthy, Start: metav1.NewTime(testStartTime)},
			{Version: "1", State: HealthStatePending, Start: metav1.NewTime(testStartTime.Add(5 * time.Minute))},
		},
		append: DeploymentStatus{
			Version: "1", State: HealthStateHealthy, Start: metav1.NewTime(testStartTime.Add(6 * time.Minute)),
		},
		want: []DeploymentStatus{
			{Version: "1", State: HealthStateHealthy, Start: metav1.NewTime(testStartTime)},
		},
	}, {
		name: "compact to unhealthy after pending gets unhealthy for the same version",
		current: []DeploymentStatus{
			{Version: "1", State: HealthStateUnhealthy, Start: metav1.NewTime(testStartTime)},
			{Version: "1", State: HealthStatePending, Start: metav1.NewTime(testStartTime.Add(5 * time.Minute))},
		},
		append: DeploymentStatus{
			Version: "1", State: HealthStateUnhealthy, Start: metav1.NewTime(testStartTime.Add(6 * time.Minute)),
		},
		want: []DeploymentStatus{
			{Version: "1", State: HealthStateUnhealthy, Start: metav1.NewTime(testStartTime)},
		},
	}, {
		name: "do not compact to healthy after pending changes to unhealthy for the same version",
		current: []DeploymentStatus{
			{Version: "1", State: HealthStateHealthy, Start: metav1.NewTime(testStartTime)},
			{Version: "1", State: HealthStatePending, Start: metav1.NewTime(testStartTime.Add(5 * time.Minute))},
		},
		append: DeploymentStatus{
			Version: "1", State: HealthStateUnhealthy, Start: metav1.NewTime(testStartTime.Add(6 * time.Minute)),
		},
		want: []DeploymentStatus{
			{Version: "1", State: HealthStateHealthy, Start: metav1.NewTime(testStartTime)},
			{Version: "1", State: HealthStateUnhealthy, Start: metav1.NewTime(testStartTime.Add(5 * time.Minute))},
		},
	}, {
		name: "do not compact to unhealthy after pending changes to healthy for the same version",
		current: []DeploymentStatus{
			{Version: "1", State: HealthStateUnhealthy, Start: metav1.NewTime(testStartTime)},
			{Version: "1", State: HealthStatePending, Start: metav1.NewTime(testStartTime.Add(5 * time.Minute))},
		},
		append: DeploymentStatus{
			Version: "1", State: HealthStateHealthy, Start: metav1.NewTime(testStartTime.Add(6 * time.Minute)),
		},
		want: []DeploymentStatus{
			{Version: "1", State: HealthStateUnhealthy, Start: metav1.NewTime(testStartTime)},
			{Version: "1", State: HealthStateHealthy, Start: metav1.NewTime(testStartTime.Add(6 * time.Minute))},
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

func TestVersionsStateDuration(t *testing.T) {
	testStartTime := time.Now()
	testCases := []struct {
		name   string
		report DeploymentStatusesReport
		want   map[string]time.Duration
	}{{
		name: "no carryover",
		report: DeploymentStatusesReport{
			DeploymentName: "prod-eu1",
			Statuses: []DeploymentStatus{{
				Start:   metav1.NewTime(testStartTime.Add(-4 * time.Hour)),
				Version: "rev-1",
				State:   HealthStateHealthy,
			}},
		},
		want: map[string]time.Duration{
			"rev-1": time.Hour * 4,
		},
	}, {
		name: "multiple healthy carryover",
		report: DeploymentStatusesReport{
			DeploymentName: "prod-eu1",
			Statuses: []DeploymentStatus{{
				Start:   metav1.NewTime(testStartTime.Add(-4 * time.Hour)),
				Version: "rev-1",
				State:   HealthStateHealthy,
			}, {
				Start:   metav1.NewTime(testStartTime.Add(-2 * time.Hour)),
				Version: "rev-2",
				State:   HealthStateHealthy,
			}, {
				Start:   metav1.NewTime(testStartTime.Add(-1 * time.Hour)),
				Version: "rev-3",
				State:   HealthStateHealthy,
			}},
		},
		want: map[string]time.Duration{
			"rev-1": time.Hour * 4,
			"rev-2": time.Hour * 2,
			"rev-3": time.Hour * 1,
		},
	}, {
		name: "last one pending",
		report: DeploymentStatusesReport{
			DeploymentName: "prod-eu1",
			Statuses: []DeploymentStatus{{
				Start:   metav1.NewTime(testStartTime.Add(-4 * time.Hour)),
				Version: "rev-1",
				State:   HealthStateHealthy,
			}, {
				Start:   metav1.NewTime(testStartTime.Add(-2 * time.Hour)),
				Version: "rev-2",
				State:   HealthStateHealthy,
			}, {
				Start:   metav1.NewTime(testStartTime.Add(-1 * time.Hour)),
				Version: "rev-3",
				State:   HealthStatePending,
			}},
		},
		want: map[string]time.Duration{
			"rev-1": time.Hour * 3,
			"rev-2": time.Hour * 1,
			"rev-3": 0,
		},
	}, {
		name: "failure on the first",
		report: DeploymentStatusesReport{
			DeploymentName: "prod-eu1",
			Statuses: []DeploymentStatus{{
				Start:   metav1.NewTime(testStartTime.Add(-4 * time.Hour)),
				Version: "rev-1",
				State:   HealthStateUnhealthy,
			}, {
				Start:   metav1.NewTime(testStartTime.Add(-2 * time.Hour)),
				Version: "rev-2",
				State:   HealthStateHealthy,
			}, {
				Start:   metav1.NewTime(testStartTime.Add(-1 * time.Hour)),
				Version: "rev-3",
				State:   HealthStateHealthy,
			}},
		},
		want: map[string]time.Duration{
			"rev-1": 0,
			"rev-2": time.Hour * 2,
			"rev-3": time.Hour * 1,
		},
	}, {
		name: "failure in the middle",
		report: DeploymentStatusesReport{
			DeploymentName: "prod-eu1",
			Statuses: []DeploymentStatus{{
				Start:   metav1.NewTime(testStartTime.Add(-4 * time.Hour)),
				Version: "rev-1",
				State:   HealthStateHealthy,
			}, {
				Start:   metav1.NewTime(testStartTime.Add(-2 * time.Hour)),
				Version: "rev-2",
				State:   HealthStateUnhealthy,
			}, {
				Start:   metav1.NewTime(testStartTime.Add(-1 * time.Hour)),
				Version: "rev-3",
				State:   HealthStateHealthy,
			}},
		},
		want: map[string]time.Duration{
			"rev-1": time.Hour * 2,
			"rev-2": time.Hour * 0,
			"rev-3": time.Hour * 1,
		},
	}, {
		name: "version fails by itself after some time",
		report: DeploymentStatusesReport{
			DeploymentName: "prod-eu1",
			Statuses: []DeploymentStatus{{
				Start:   metav1.NewTime(testStartTime.Add(-4 * time.Hour)),
				Version: "rev-1",
				State:   HealthStateHealthy,
			}, {
				Start:   metav1.NewTime(testStartTime.Add(-2 * time.Hour)),
				Version: "rev-1",
				State:   HealthStateUnhealthy,
			}},
		},
		want: map[string]time.Duration{
			"rev-1": time.Hour * 0,
		},
	}, {
		name: "version health flapping",
		report: DeploymentStatusesReport{
			DeploymentName: "prod-eu1",
			Statuses: []DeploymentStatus{{
				Start:   metav1.NewTime(testStartTime.Add(-4 * time.Hour)),
				Version: "rev-1",
				State:   HealthStateHealthy,
			}, {
				Start:   metav1.NewTime(testStartTime.Add(-3 * time.Hour)),
				Version: "rev-1",
				State:   HealthStateUnhealthy,
			}, {
				Start:   metav1.NewTime(testStartTime.Add(-2 * time.Hour)),
				Version: "rev-1",
				State:   HealthStateHealthy,
			}},
		},
		want: map[string]time.Duration{
			"rev-1": time.Hour * 0,
		},
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for v, d := range tc.want {
				adjustedResult := (tc.report.VersionHealthyDuration(v) - d).Abs() - time.Since(testStartTime)
				assert.True(t, adjustedResult < 0)
				assert.True(t, adjustedResult > -100*time.Millisecond)
			}
		})
	}
}

func TestNextVersion(t *testing.T) {
	testStartTime := time.Now()
	testCases := []struct {
		name        string
		propagation Propagation
		want        string
	}{{
		name: "no candidate",
		propagation: Propagation{
			Spec: PropagationSpec{},
			Status: PropagationStatus{
				DeployConditions: DeployConditions{

					DeployAfter: DeployAfter{
						Deployments: []string{
							"prod-eu1",
						},
						BakeTime: metav1.Duration{
							Duration: time.Hour * 4,
						},
					},
				},
				DeploymentStatusesReports: []DeploymentStatusesReport{{
					DeploymentName: "prod-eu1",
					Statuses: []DeploymentStatus{{
						Start:   metav1.NewTime(testStartTime),
						Version: "rev-1",
						State:   HealthStateHealthy,
					}},
				}}},
		},
	}, {
		name: "single candidate valid",
		propagation: Propagation{
			Spec: PropagationSpec{},
			Status: PropagationStatus{
				DeployConditions: DeployConditions{
					DeployAfter: DeployAfter{
						Deployments: []string{
							"prod-eu1",
						},
						BakeTime: metav1.Duration{
							Duration: time.Hour * 4,
						},
					},
				},
				DeploymentStatusesReports: []DeploymentStatusesReport{{
					DeploymentName: "prod-eu1",
					Statuses: []DeploymentStatus{{
						Start:   metav1.NewTime(testStartTime.Add(-time.Hour * 4)),
						Version: "rev-1",
						State:   HealthStateHealthy,
					}},
				}}},
		},
		want: "rev-1",
	}, {
		name: "extra non-relevant statuses should be ignored",
		propagation: Propagation{
			Spec: PropagationSpec{},
			Status: PropagationStatus{
				DeployConditions: DeployConditions{
					DeployAfter: DeployAfter{
						Deployments: []string{
							"prod-eu1",
						},
						BakeTime: metav1.Duration{
							Duration: time.Hour * 4,
						},
					},
				},
				DeploymentStatusesReports: []DeploymentStatusesReport{{
					DeploymentName: "prod-other",
					Statuses: []DeploymentStatus{{
						Start:   metav1.NewTime(testStartTime.Add(-time.Minute * 4)),
						Version: "rev-3",
						State:   HealthStateHealthy,
					}},
				}, {
					DeploymentName: "prod-eu1",
					Statuses: []DeploymentStatus{{
						Start:   metav1.NewTime(testStartTime.Add(-time.Hour * 4)),
						Version: "rev-1",
						State:   HealthStateHealthy,
					}},
				}}},
		},
		want: "rev-1",
	}, {
		name: "one candidate valid, one invalid",
		propagation: Propagation{
			Spec: PropagationSpec{},
			Status: PropagationStatus{
				DeployConditions: DeployConditions{
					DeployAfter: DeployAfter{
						Deployments: []string{
							"prod-eu1",
						},
						BakeTime: metav1.Duration{
							Duration: time.Hour * 4,
						},
					},
				},
				DeploymentStatusesReports: []DeploymentStatusesReport{{
					DeploymentName: "prod-eu1",
					Statuses: []DeploymentStatus{{
						Start:   metav1.NewTime(testStartTime.Add(-time.Hour * 4)),
						Version: "rev-1",
						State:   HealthStateHealthy,
					}, {
						Start:   metav1.NewTime(testStartTime.Add(-time.Hour * 2)),
						Version: "rev-2",
						State:   HealthStateHealthy,
					}},
				}}},
		},
		want: "rev-1",
	}, {
		name: "first wave",
		want: "latest",
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.propagation.NextVersion())
		})
	}
}
