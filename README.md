[![Build Status](https://travis-ci.org/gmauleon/alertmanager-zabbix-provisioner.svg?branch=master)](https://travis-ci.org/gmauleon/alertmanager-zabbix-provisioner)

## alertmanager-zabbix-provisioner

The provisioner will connect to your prometheus to get the current configured rules and will create a host/items/triggers accordingly  
Data for those items can then be sent via a webhook: https://github.com/gmauleon/alertmanager-zabbix-webhook

## Howto

Have a look at the default [config.yaml](https://github.com/gmauleon/alertmanager-zabbix-provisioner/blob/master/config.yaml) for the possible parameters  
Kubernetes examples manifests can be found here: https://github.com/gmauleon/alertmanager-zabbix-webhook/tree/master/contrib/kubernetes  

To create a corresponding item/trigger in Zabbix, your prometheus rule need to have at least a `zabbix` annotation:
```
ANNOTATIONS {
  zabbix = "true",
  summary = "Alerting DeadMansSwitch",
  description = "This is a DeadMansSwitch meant to ensure that the entire Alerting pipeline is functional.",
}
```
If the annotation is not present the rule will be ignored, you can choose what rule you want in Zabbixi with that  

Fields for an item are populated (from left to right if annotation exists):  
Name = rule Name  
Description = `zabbix_description` annotation, `description` annotation, empty  
Applications = `zabbix_application` annotation, empty  
History storage period = `zabbix_history` annotation, 7d  
Trend storage period = `zabbix_trend` annotation, 90d  

Fields for triggers are populated (from left to right if annotation exists):  
Name = `zabbix_trigger_name` annotation, `summary` annotation, rule name  
Description = `zabbix_trigger_description` annotation, `description` annotation, empty  
Severity = `zabbix_trigger_severity` annotation, not classified  

Examples:
```
ANNOTATIONS {
  zabbix = "true",
  zabbix_application = "kubelet",
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
