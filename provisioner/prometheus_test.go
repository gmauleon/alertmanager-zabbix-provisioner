package provisioner

import (
	"github.com/stretchr/testify/assert"
	"gopkg.in/h2non/gock.v1"
	"io/ioutil"
	"testing"
)

var expectedAnswers = []PrometheusRule{
	{"AlertmanagerConfigInconsistent", map[string]string{}},
	{"AlertmanagerFailedReload", map[string]string{}},
	{"AlertmanagerMembersInconsistent", map[string]string{}},
	{"KubePodCrashLooping", map[string]string{}},
	{"KubePodNotReady", map[string]string{}},
	{"KubeDeploymentGenerationMismatch", map[string]string{}},
	{"KubeDeploymentReplicasMismatch", map[string]string{}},
	{"KubeStatefulSetReplicasMismatch", map[string]string{}},
	{"KubeStatefulSetGenerationMismatch", map[string]string{}},
	{"KubeStatefulSetUpdateNotRolledOut", map[string]string{}},
	{"KubeDaemonSetRolloutStuck", map[string]string{}},
	{"KubeDaemonSetNotScheduled", map[string]string{}},
	{"KubeDaemonSetMisScheduled", map[string]string{}},
	{"TargetDown", map[string]string{}},
	{"Watchdog", map[string]string{}},
	{"NodeDiskRunningFull", map[string]string{}},
	{"NodeDiskRunningFull", map[string]string{}},
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
