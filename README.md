[![Build Status](https://travis-ci.org/gmauleon/alertmanager-zabbix-provisioner.svg?branch=master)](https://travis-ci.org/gmauleon/alertmanager-zabbix-provisioner)

## alertmanager-zabbix-provisioner

The provisioner will connect to your prometheus to get the current configured rules and will create a host/items/triggers accordingly  
Data for those items can then be sent via a webhook: https://github.com/gmauleon/alertmanager-zabbix-webhook

The concept is to use annotations in prometheus rules, along with a provisionner configuration file to automatically create everything in Zabbix

## Howto

Have a look at the default [config.yaml](https://github.com/gmauleon/alertmanager-zabbix-provisioner/blob/master/config.yaml) for the possible parameters  
Kubernetes examples manifests can be found here: https://github.com/gmauleon/alertmanager-zabbix-webhook/tree/master/contrib/kubernetes  

To create a host with items/triggers in Zabbix, your prometheus rule need to have some annotations matching your `selector` configuration for that host  
For example that configuration:
```
zabbixHosts:
  - name: gmauleon-test01
    selector:
      zabbix: gmauleon-test01
```

Will match rules like:
```
ANNOTATIONS {
  zabbix = "gmauleon-test01",
  summary = "Alerting DeadMansSwitch",
  description = "This is a DeadMansSwitch meant to ensure that the entire Alerting pipeline is functional.",
}
```

You can choose what Prometheus rule appear in Zabbix with that behavior.  
In Zabbix, fields for an item are populated following the behavior below:  

Name = rule name  
Description = `zabbix_description` annotation OR `description` annotation OR empty  
Applications = `zabbix_applications` annotation OR `itemDefaultApplication` configuration  
History storage period = `zabbix_history` annotation OR  `itemDefaultHistory` configuration  
Trend storage period = `zabbix_trend` annotation OR `itemDefaultTrends` configuration  
Allowed hosts = `zabbix_trapper_hosts` annotation OR `itemDefaultTrapperHosts` configuration  


Fields for triggers are populated:  

Name = `zabbix_trigger_name` annotation OR `summary` annotation OR rule name  
Description = `zabbix_trigger_description` annotation OR `description` annotation OR empty  
Severity = `zabbix_trigger_severity` annotation OR `not classified`  

Examples in prometheus:
```
ANNOTATIONS {
  zabbix = "gmauleon-test01",
  zabbix_history = "7d",
  zabbix_applications = "kubelet,prometheus,alertmanager",
  zabbix_trigger_severity = "critical",
  zabbix_description = "One of the Kubelet has not checked in with the API",
  summary = "Node status is NotReady",
  description = "The Kubelet on {{ $labels.node }} has not checked in with the API, or has set itself to NotReady, for more than an hour",
}
```

## Limitations
For now a minimal scraper, parse the html page that expose rules on your Prometheus (which is pretty clumsy :( )  
Note that the HTML page exposed by prometheus currently does not resolve variables, so it's better to use the dedicated Zabbix annotations not to have variables names in your Zabbix items...  
Ultimately, this will be replaced by the rules API endpoint, see https://github.com/prometheus/prometheus/pull/2600  

Since host groups and hosts are declared in the provisionner configuration, there will not be deleted automatically (since I don't have any state saved anywhere).  
So youll have to delete those by hands in Zabbix if you remove some


