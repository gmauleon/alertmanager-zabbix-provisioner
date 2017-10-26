package provisioner

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/gmauleon/zabbix-client"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"time"
)

var log = logrus.WithField("context", "provisioner")

type Provisioner struct {
	Api *zabbix.API
	ProvisionerConfig
}

type ProvisionerConfig struct {
	RulesUrl                      string   `yaml:"rulesUrl"`
	RulesPollingInterval          int      `yaml:"rulesPollingTime"`
	ZabbixApiUrl                  string   `yaml:"zabbixApiUrl"`
	ZabbixApiCAFile               string   `yaml:"zabbixApiCAFile"`
	ZabbixApiUser                 string   `yaml:"zabbixApiUser"`
	ZabbixApiPassword             string   `yaml:"zabbixApiPassword"`
	ZabbixKeyPrefix               string   `yaml:"zabbixKeyPrefix"`
	ZabbixHost                    string   `yaml:"zabbixHost"`
	ZabbixHostGroups              []string `yaml:"zabbixHostGroups"`
	ZabbixItemDefaultHistory      string   `yaml:"zabbixItemDefaultHistory"`
	ZabbixItemDefaultTrends       string   `yaml:"zabbixItemDefaultTrends"`
	ZabbixItemDefaultTrapperHosts string   `yaml:"zabbixItemDefaultTrapperHosts"`
}

func New(cfg *ProvisionerConfig) *Provisioner {

	// Use the correct CA bundle if provided
	transport := http.DefaultTransport
	if len(cfg.ZabbixApiCAFile) != 0 {
		// Add custom CA certificate
		caCert, err := ioutil.ReadFile(cfg.ZabbixApiCAFile)
		if err != nil {
			log.Fatal(err)
		}

		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)

		tlsConfig := &tls.Config{}
		tlsConfig.RootCAs = caCertPool
		transport = &http.Transport{TLSClientConfig: tlsConfig}
	}

	api := zabbix.NewAPI(cfg.ZabbixApiUrl)
	api.SetClient(&http.Client{
		Transport: transport,
	})

	_, err := api.Login(cfg.ZabbixApiUser, cfg.ZabbixApiPassword)
	if err != nil {
		log.Fatal(err)
	}

	return &Provisioner{
		Api:               api,
		ProvisionerConfig: *cfg,
	}

}

func ConfigFromFile(filename string) (cfg *ProvisionerConfig, err error) {
	log.Infof("Loading configuration at '%s'", filename)
	configFile, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("Can't open the config file: %s", err)
	}

	// Default values
	config := ProvisionerConfig{
		RulesUrl:             "https://127.0.0.1/prometheus/rules",
		RulesPollingInterval: 3600,
		ZabbixApiUrl:         "https://127.0.0.1/zabbix/api_jsonrpc.php",
		ZabbixApiUser:        "user",
		ZabbixApiPassword:    "password",
		ZabbixHost:           "kubernetes-cluster-prometheus",
		ZabbixKeyPrefix:      "prometheus",
		ZabbixHostGroups: []string{
			"Kubernetes",
			"Prometheus",
		},
		ZabbixItemDefaultHistory:      "7d",
		ZabbixItemDefaultTrends:       "90d",
		ZabbixItemDefaultTrapperHosts: "0.0.0.0/32",
	}

	err = yaml.Unmarshal(configFile, &config)
	if err != nil {
		return nil, fmt.Errorf("Can't read the config file: %s", err)
	}

	log.Info("Configuration loaded")

	// If Environment variables are set for zabbix user and password, use those instead
	zabbixApiUser, ok := os.LookupEnv("ZABBIX_API_USER")
	if ok {
		config.ZabbixApiUser = zabbixApiUser
	}

	zabbixApiPassword, ok := os.LookupEnv("ZABBIX_API_PASSWORD")
	if ok {
		config.ZabbixApiPassword = zabbixApiPassword
	}

	return &config, nil
}

func (p *Provisioner) Start() {

	for {

		rules := GetRulesFromURL(p.RulesUrl)
		// TODO
		// TODO: Compare rules and do something only if there is some changes
		// TODO

		hostGroups := p.createHostGroups()
		host := p.createHost(hostGroups)

		zabbixItems, err := p.Api.ItemsGet(zabbix.Params{
			"output":  "extend",
			"hostids": host.HostId,
		})

		if err != nil {
			log.Fatal(err)
		}

		wantedItems := p.getItemsFromPrometheusRules(host, rules)
		existingItems := p.getItemsFromZabbixItems(host, zabbixItems)
		p.syncItems(wantedItems, existingItems)

		time.Sleep(time.Duration(p.RulesPollingInterval) * time.Second)
	}
}

func (p *Provisioner) createHostGroups() zabbix.HostGroups {

	// Get exising hot groups from Zabbix
	existingHostGroups, err := p.Api.HostGroupsGet(zabbix.Params{
		"output": "extend",
		"filter": map[string][]string{
			"name": p.ZabbixHostGroups,
		},
	})

	if err != nil {
		log.Fatal(err)
	}

	if len(existingHostGroups) == len(p.ZabbixHostGroups) {
		log.Info("Host Groups exists")
		return existingHostGroups
	}

	newHostGroups := zabbix.HostGroups{}
	for _, name := range p.ZabbixHostGroups {
		found := false
		for _, h := range existingHostGroups {
			if h.Name == name {
				found = true
				break
			}
		}

		if !found {
			newHostGroups = append(newHostGroups, zabbix.HostGroup{Name: name})
		}
	}

	// Create missing host groups
	err = p.Api.HostGroupsCreate(newHostGroups)
	if err != nil {
		log.Fatal(err)
	}

	log.Info("Host Groups created")
	return append(existingHostGroups, newHostGroups...)
}

func (p *Provisioner) createHost(hg zabbix.HostGroups) zabbix.Host {

	existingHosts, err := p.Api.HostsGet(zabbix.Params{
		"output": "extend",
		"filter": map[string]string{
			"host": p.ZabbixHost,
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	if len(existingHosts) != 0 {
		log.Info("Host exists")
		return existingHosts[0]
	}

	newHosts := zabbix.Hosts{
		zabbix.Host{
			Host:      p.ZabbixHost,
			Available: 1,
			Name:      p.ZabbixHost,
			Status:    0,
			GroupIds:  hostGroupIds(hg),
			Interfaces: zabbix.HostInterfaces{
				zabbix.HostInterface{
					DNS:   "",
					IP:    "127.0.0.1",
					Main:  1,
					Port:  "10050",
					Type:  1,
					UseIP: 1,
				},
			},
		},
	}

	err = p.Api.HostsCreate(newHosts)
	if err != nil {
		log.Fatal(err)
	}

	log.Info("Host created")
	return newHosts[0]
}

func (p *Provisioner) getItemsFromPrometheusRules(host zabbix.Host, rules []PrometheusRule) Items {
	var items Items
	for _, r := range rules {
		if _, ok := r.Annotations["zabbix"]; ok {
			item := NewFromPrometheusRule(r, host, p.ZabbixItemDefaultHistory, p.ZabbixItemDefaultTrends, p.ZabbixItemDefaultTrapperHosts)
			items = append(items, *item)
			log.Infof("Item from Prometheus: %s", item.Item.Name)
		}
	}
	sort.Sort(items)
	return items
}

func (p *Provisioner) getItemsFromZabbixItems(host zabbix.Host, zabbixItems zabbix.Items) Items {
	var items Items
	for _, i := range zabbixItems {
		applications, err := p.Api.ApplicationsGet(zabbix.Params{
			"output":  "extend",
			"hostids": host.HostId,
			"itemids": i.ItemId,
		})

		if err != nil {
			log.Errorf("Can't find applications for item '%s'", i.Name)
		}

		triggers, err := p.Api.TriggersGet(zabbix.Params{
			"output":  "extend",
			"hostids": host.HostId,
			"itemids": i.ItemId,
		})

		if err != nil {
			log.Fatalf("Can't find triggers for item '%s'", i.Name)
		}

		item := NewFromZabbixItem(i, applications, triggers[0])
		items = append(items, *item)
		log.Infof("Item from Zabbix: %s", item.Item.Name)
	}

	sort.Sort(items)
	return items
}

func (p *Provisioner) syncItems(wantedItems Items, existingItems Items) {

	itemsToCreate := Items{}
	itemsToDelete := Items{}

	i, j := 0, 0
	for i < len(wantedItems) && j < len(existingItems) {

		nameResult, otherResult := wantedItems[i].Compare(existingItems[j])
		if nameResult < 0 {
			itemsToCreate = append(itemsToCreate, wantedItems[i])
			i++
		} else if nameResult > 0 {
			itemsToDelete = append(itemsToDelete, existingItems[j])
			j++
		} else {
			if !otherResult {
				itemsToCreate = append(itemsToCreate, wantedItems[i])
				itemsToDelete = append(itemsToDelete, existingItems[j])
			}
			j++
			i++
		}
	}

	if i < len(wantedItems) {
		itemsToCreate = append(itemsToCreate, wantedItems[i:]...)
	} else {
		itemsToDelete = append(itemsToDelete, existingItems[j:]...)
	}

	if len(itemsToDelete) != 0 {
		for _, i := range itemsToDelete {
			log.Infof("Item to delete in Zabbix: %s", i.Item.Name)
		}
		err := p.Api.ItemsDelete(itemsToDelete.Items())
		if err != nil {
			log.Fatal(err)
		}
	} else {
		log.Info("Nothing to delete")
	}

	if len(itemsToCreate) != 0 {
		for _, i := range itemsToCreate {
			log.Infof("Item to create in Zabbix: %s", i.Item.Name)
		}
		err := p.Api.ItemsCreate(itemsToCreate.Items())
		if err != nil {
			log.Fatalf("Problem while creating items: %s", err)
		}
		err = p.Api.TriggersCreate(itemsToCreate.Triggers())
		if err != nil {
			log.Fatalf("Problem while creating triggers: %s", err)
		}
	} else {
		log.Info("Nothing to create")
	}

}

func hostGroupIds(hg zabbix.HostGroups) zabbix.HostGroupIds {
	ids := make([]zabbix.HostGroupId, len(hg))
	for i, group := range hg {
		ids[i] = zabbix.HostGroupId{group.GroupId}
	}
	return ids
}
