package provisioner

import (
	"github.com/stretchr/testify/assert"
	"gopkg.in/h2non/gock.v1"
	"io/ioutil"
	"testing"
)

var expectedAnswers = []PrometheusRule{
	{"AlertmanagerConfigInconsistent",
		map[string]string{"message": "The configuration of the instances of the Alertmanager cluster `{{$labels.service}}` are out of sync."}},
	{"AlertmanagerFailedReload",
		map[string]string{"message": "Reloading Alertmanager's configuration has failed for {{ $labels.namespace }}/{{ $labels.pod}}."}},
	{"AlertmanagerMembersInconsistent",
		map[string]string{"message": "Alertmanager has not found all other members of the cluster."}},
	{"KubePodCrashLooping",
		map[string]string{"message": "Pod {{ $labels.namespace }}/{{ $labels.pod }} ({{ $labels.container }}) is restarting {{ printf \"%.2f\" $value }} times / 5 minutes.",
			"runbook_url": "https://github.com/kubernetes-monitoring/kubernetes-mixin/tree/master/runbook.md#alert-name-kubepodcrashlooping"}},
	{"KubePodNotReady",
		map[string]string{"message": "Pod {{ $labels.namespace }}/{{ $labels.pod }} has been in a non-ready state for longer than an hour.",
			"runbook_url": "https://github.com/kubernetes-monitoring/kubernetes-mixin/tree/master/runbook.md#alert-name-kubepodnotready"}},
	{"KubeDeploymentGenerationMismatch",
		map[string]string{"message": "Deployment generation for {{ $labels.namespace }}/{{ $labels.deployment }} does not match, this indicates that the Deployment has failed but has not been rolled back.",
			"runbook_url": "https://github.com/kubernetes-monitoring/kubernetes-mixin/tree/master/runbook.md#alert-name-kubedeploymentgenerationmismatch"}},
	{"KubeDeploymentReplicasMismatch",
		map[string]string{"message": "Deployment {{ $labels.namespace }}/{{ $labels.deployment }} has not matched the expected number of replicas for longer than an hour.",
			"runbook_url": "https://github.com/kubernetes-monitoring/kubernetes-mixin/tree/master/runbook.md#alert-name-kubedeploymentreplicasmismatch"}},
	{"KubeStatefulSetReplicasMismatch",
		map[string]string{"message": "StatefulSet {{ $labels.namespace }}/{{ $labels.statefulset }} has not matched the expected number of replicas for longer than 15 minutes.",
			"runbook_url": "https://github.com/kubernetes-monitoring/kubernetes-mixin/tree/master/runbook.md#alert-name-kubestatefulsetreplicasmismatch"}},
	{"KubeStatefulSetGenerationMismatch",
		map[string]string{"message": "StatefulSet generation for {{ $labels.namespace }}/{{ $labels.statefulset }} does not match, this indicates that the StatefulSet has failed but has not been rolled back.",
			"runbook_url": "https://github.com/kubernetes-monitoring/kubernetes-mixin/tree/master/runbook.md#alert-name-kubestatefulsetgenerationmismatch"}},
	{"KubeStatefulSetUpdateNotRolledOut",
		map[string]string{"message": "StatefulSet {{ $labels.namespace }}/{{ $labels.statefulset }} update has not been rolled out.",
			"runbook_url": "https://github.com/kubernetes-monitoring/kubernetes-mixin/tree/master/runbook.md#alert-name-kubestatefulsetupdatenotrolledout"}},
	{"KubeDaemonSetRolloutStuck",
		map[string]string{"message": "Only {{ $value }}% of the desired Pods of DaemonSet {{ $labels.namespace }}/{{ $labels.daemonset }} are scheduled and ready.",
			"runbook_url": "https://github.com/kubernetes-monitoring/kubernetes-mixin/tree/master/runbook.md#alert-name-kubedaemonsetrolloutstuck"}},
	{"KubeDaemonSetNotScheduled",
		map[string]string{"message": "'{{ $value }} Pods of DaemonSet {{ $labels.namespace }}/{{ $labels.daemonset }} are not scheduled.'",
			"runbook_url": "https://github.com/kubernetes-monitoring/kubernetes-mixin/tree/master/runbook.md#alert-name-kubedaemonsetnotscheduled"}},
	{"KubeDaemonSetMisScheduled",
		map[string]string{"message": "'{{ $value }} Pods of DaemonSet {{ $labels.namespace }}/{{ $labels.daemonset }} are running where they are not supposed to run.'",
			"runbook_url": "https://github.com/kubernetes-monitoring/kubernetes-mixin/tree/master/runbook.md#alert-name-kubedaemonsetmisscheduled"}},
	{"TargetDown",
		map[string]string{"message": "'{{ $value }}% of the {{ $labels.job }} targets are down.'"}},
	{"Watchdog",
		map[string]string{"message": "This is an alert meant to ensure that the entire alerting pipeline is functional. This alert is always firing, therefore it should always be firing in Alertmanager and always fire against a receiver. There are integrations with various notification mechanisms that send a notification when this alert is not firing. For example the \"DeadMansSnitch\" integration in PagerDuty."}},
	{"NodeDiskRunningFull",
		map[string]string{"message": "Device {{ $labels.device }} of node-exporter {{ $labels.namespace }}/{{ $labels.pod }} will be full within the next 24 hours."}},
	{"NodeDiskRunningFull",
		map[string]string{"message": "Device {{ $labels.device }} of node-exporter {{ $labels.namespace }}/{{ $labels.pod }} will be full within the next 2 hours."}},
}

func TestDataRetrieval(t *testing.T) {
	defer gock.Off()
	f, _ := ioutil.ReadFile("prometheus_rules_test_body.txt")

	gock.New("http://prometheus").
		Get("/rules").
		Reply(200).
		BodyString(string(f))

	rules := GetRulesFromURL("http://prometheus:9090/rules")
	assert.EqualValues(t, expectedAnswers, rules, "rules read from Prometheus are parsed correctly")
}
