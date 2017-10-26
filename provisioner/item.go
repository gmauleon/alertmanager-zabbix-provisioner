package provisioner

import (
	"fmt"
	"github.com/gmauleon/zabbix-client"
	"strings"
)

type Items []Item

type Item struct {
	Item         zabbix.Item
	Applications zabbix.Applications
	Trigger      zabbix.Trigger
}

func (i Item) Compare(j Item) (resultName int, resultOther bool) {
	if i.Item.Name < j.Item.Name {
		return -1, false
	}

	if i.Item.Name > j.Item.Name {
		return 1, false
	}

	if i.Item.Description != j.Item.Description {
		return 0, false
	}

	if i.Item.Trends != j.Item.Trends {
		return 0, false
	}

	if i.Item.History != j.Item.History {
		return 0, false
	}

	if i.Item.TrapperHosts != j.Item.TrapperHosts {
		return 0, false
	}

	// Trigger comparison
	if i.Trigger.Description != j.Trigger.Description {
		return 0, false
	}

	if i.Trigger.Priority != j.Trigger.Priority {
		return 0, false
	}

	if i.Trigger.Comments != j.Trigger.Comments {
		return 0, false
	}

	if len(i.Applications) != len(j.Applications) {
		return 0, false
	}

	for _, ia := range i.Applications {
		found := false
		for _, ja := range j.Applications {
			if ia.Name == ja.Name {
				found = true
				break
			}
		}
		if found == false {
			return 0, false
		}
	}

	return 0, true
}

func NewFromPrometheusRule(rule PrometheusRule, host zabbix.Host, history string, trends string, trapperHosts string) *Item {
	key := fmt.Sprintf("prometheus.%s", strings.ToLower(rule.Name))

	newItem := Item{
		Item: zabbix.Item{
			Name:         rule.Name,
			Key:          key,
			HostId:       host.HostId,
			Type:         2, //Trapper
			ValueType:    3,
			History:      history,
			Trends:       trends,
			TrapperHosts: trapperHosts,
		},
		Trigger: zabbix.Trigger{
			Description: rule.Name,
			Expression:  fmt.Sprintf("{%s:%s.last()}<>0", host.Name, key),
		},
	}

	for k, v := range rule.Annotations {
		switch k {
		case "zabbix_application":
			newItem.Applications = zabbix.Applications{
				zabbix.Application{
					Name: v,
				},
			}
		case "description":
			if _, ok := rule.Annotations["zabbix_description"]; !ok {
				newItem.Item.Description = v
			}

			// Description is called Comments in Zabbix API
			if _, ok := rule.Annotations["zabbix_trigger_description"]; !ok {
				newItem.Trigger.Comments = v
			}
		case "zabbix_description":
			newItem.Item.Description = v
		case "zabbix_history":
			newItem.Item.History = v
		case "zabbix_trend":
			newItem.Item.Trends = v
		case "zabbix_trapper_hosts":
			newItem.Item.TrapperHosts = v
		case "summary":
			// Trigger name is called description in Zabbix API
			if _, ok := rule.Annotations["zabbix_trigger_name"]; !ok {
				newItem.Trigger.Description = v
			}
		case "zabbix_trigger_name":
			newItem.Trigger.Description = v
		case "zabbix_trigger_description":
			newItem.Trigger.Comments = v
		case "zabbix_trigger_severity":
			newItem.Trigger.Priority = GetZabbixPriority(v)
		default:
			continue
		}
	}

	return &newItem
}

func NewFromZabbixItem(i zabbix.Item, a zabbix.Applications, t zabbix.Trigger) *Item {
	newItem := Item{
		Item:         i,
		Applications: a,
		Trigger:      t,
	}

	return &newItem
}

func GetZabbixPriority(severity string) zabbix.PriorityType {

	switch strings.ToLower(severity) {
	case "information":
		return zabbix.Information
	case "warning":
		return zabbix.Warning
	case "average":
		return zabbix.Average
	case "high":
		return zabbix.High
	case "critical":
		return zabbix.Critical
	default:
		return zabbix.NotClassified
	}
}

func (z Items) Len() int      { return len(z) }
func (z Items) Swap(i, j int) { z[i], z[j] = z[j], z[i] }
func (z Items) Less(i, j int) bool {
	return z[i].Item.Name < z[j].Item.Name
}

func (z Items) Items() zabbix.Items {
	items := zabbix.Items{}
	for _, i := range z {
		items = append(items, i.Item)
	}
	return items
}

func (z Items) Triggers() zabbix.Triggers {
	triggers := zabbix.Triggers{}
	for _, i := range z {
		triggers = append(triggers, i.Trigger)
	}
	return triggers
}
