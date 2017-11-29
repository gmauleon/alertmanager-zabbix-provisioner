package provisioner

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/gmauleon/zabbix-client"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

type Provisioner struct {
	Api    *zabbix.API
	Config ProvisionerConfig
	*CustomZabbix
}

type ProvisionerConfig struct {
	RulesUrl             string `yaml:"rulesUrl"`
	RulesPollingInterval int    `yaml:"rulesPollingTime"`

	ZabbixApiUrl      string       `yaml:"zabbixApiUrl"`
	ZabbixApiCAFile   string       `yaml:"zabbixApiCAFile"`
	ZabbixApiUser     string       `yaml:"zabbixApiUser"`
	ZabbixApiPassword string       `yaml:"zabbixApiPassword"`
	ZabbixKeyPrefix   string       `yaml:"zabbixKeyPrefix"`
	ZabbixHosts       []HostConfig `yaml:"zabbixHosts"`
}

type HostConfig struct {
	Name                    string            `yaml:"name"`
	Selector                map[string]string `yaml:"selector"`
	HostGroups              []string          `yaml:"hostGroups"`
	Tag                     string            `yaml:"tag"`
	DeploymentStatus        string            `yaml:"deploymentStatus"`
	ItemDefaultApplication  string            `yaml:"itemDefaultApplication"`
	ItemDefaultHistory      string            `yaml:"itemDefaultHistory"`
	ItemDefaultTrends       string            `yaml:"itemDefaultTrends"`
	ItemDefaultTrapperHosts string            `yaml:"itemDefaultTrapperHosts"`
}

func New(cfg *ProvisionerConfig) *Provisioner {

	// Use the correct CA bundle if provided
	transport := http.DefaultTransport
	if len(cfg.ZabbixApiCAFile) != 0 {
		// Add custom CA certificate
		caCert, err := ioutil.ReadFile(cfg.ZabbixApiCAFile)
		if err != nil {
			log.Fatalln("error while reading Zabbix CA: ", err)
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
		log.Fatalln("error while login to Zabbix:", err)
	}

	return &Provisioner{
		Api:    api,
		Config: *cfg,
	}

}

func ConfigFromFile(filename string) (cfg *ProvisionerConfig, err error) {
	log.Infof("loading configuration at '%s'", filename)
	configFile, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("can't open the config file: %s", err)
	}

	// Default values
	config := ProvisionerConfig{
		RulesUrl:             "https://127.0.0.1/prometheus/rules",
		RulesPollingInterval: 3600,
		ZabbixApiUrl:         "https://127.0.0.1/zabbix/api_jsonrpc.php",
		ZabbixApiUser:        "user",
		ZabbixApiPassword:    "password",
		ZabbixKeyPrefix:      "prometheus",
		ZabbixHosts:          []HostConfig{},
	}

	err = yaml.Unmarshal(configFile, &config)
	if err != nil {
		return nil, fmt.Errorf("can't read the config file: %s", err)
	}

	log.Info("configuration loaded")

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

		// TODO
		// TODO: Compare rules and do something only if there is some changes
		// TODO

		p.CustomZabbix = NewCustomZabbix()

		p.FillFromPrometheus()
		p.FillFromZabbix()
		p.ApplyChanges()

		time.Sleep(time.Duration(p.Config.RulesPollingInterval) * time.Second)
	}
}

// Check if a prometheus rule has all the key/value pair declared in the selector configuration for a host
func (p *Provisioner) IsMatching(config HostConfig, rule PrometheusRule) bool {

	if len(config.Selector) == 0 {
		return false
	}

	for hostKey, hostValue := range config.Selector {
		if ruleValue, ok := rule.Annotations[hostKey]; ok {
			if hostValue != ruleValue {
				return false
			}
		} else {
			return false
		}
	}
	return true
}

// Create hosts structures and populate them from Prometheus rules
func (p *Provisioner) FillFromPrometheus() {

	rules := GetRulesFromURL(p.Config.RulesUrl)

	for _, hostConfig := range p.Config.ZabbixHosts {

		// Create an internal host object
		newHost := &CustomHost{
			State: StateNew,
			Host: zabbix.Host{
				Host:          hostConfig.Name,
				Available:     1,
				Name:          hostConfig.Name,
				Status:        0,
				InventoryMode: zabbix.InventoryManual,
				Inventory: map[string]string{
					"deployment_status": hostConfig.DeploymentStatus,
					"tag":               hostConfig.Tag,
				},
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
			HostGroups:   make(map[string]struct{}, len(hostConfig.HostGroups)),
			Items:        map[string]*CustomItem{},
			Applications: map[string]*CustomApplication{},
			Triggers:     map[string]*CustomTrigger{},
		}

		// Create host groups from the configuration file and link them to this host
		for _, hostGroupName := range hostConfig.HostGroups {
			p.AddHostGroup(&CustomHostGroup{
				State: StateNew,
				HostGroup: zabbix.HostGroup{
					Name: hostGroupName,
				},
			})

			newHost.HostGroups[hostGroupName] = struct{}{}
		}

		// Parse Prometheus rules and create corresponding items/triggers and applications for this host
		for _, rule := range rules {

			if !p.IsMatching(hostConfig, rule) {
				continue
			}

			key := fmt.Sprintf("prometheus.%s", strings.ToLower(rule.Name))

			newItem := &CustomItem{
				State: StateNew,
				Item: zabbix.Item{
					Name:         rule.Name,
					Key:          key,
					HostId:       "", //To be filled when the host will be created
					Type:         2,  //Trapper
					ValueType:    3,
					History:      hostConfig.ItemDefaultHistory,
					Trends:       hostConfig.ItemDefaultTrends,
					TrapperHosts: hostConfig.ItemDefaultTrapperHosts,
				},
				Applications: map[string]struct{}{},
			}

			newTrigger := &CustomTrigger{
				State: StateNew,
				Trigger: zabbix.Trigger{
					Description: rule.Name,
					Expression:  fmt.Sprintf("{%s:%s.last()}<>0", newHost.Name, key),
				},
			}

			for k, v := range rule.Annotations {
				switch k {
				case "zabbix_applications":

					// List of applications separated by comma
					applicationNames := strings.Split(v, ",")
					for _, applicationName := range applicationNames {
						newApplication := &CustomApplication{
							State: StateNew,
							Application: zabbix.Application{
								Name: applicationName,
							},
						}

						newHost.AddApplication(newApplication)

						if _, ok := newItem.Applications[applicationName]; !ok {
							newItem.Applications[applicationName] = struct{}{}
						}
					}
				case "description":
					// If a specific description for this item is not present use the default prometheus description
					if _, ok := rule.Annotations["zabbix_description"]; !ok {
						newItem.Description = v
					}

					// If a specific description for this trigger is not present use the default prometheus description
					// Note that trigger "description" are called "comments" in the Zabbix API
					if _, ok := rule.Annotations["zabbix_trigger_description"]; !ok {
						newTrigger.Comments = v
					}
				case "zabbix_description":
					newItem.Description = v
				case "zabbix_history":
					newItem.History = v
				case "zabbix_trend":
					newItem.Trends = v
				case "zabbix_trapper_hosts":
					newItem.TrapperHosts = v
				case "summary":
					// Note that trigger "name" is called "description" in the Zabbix API
					if _, ok := rule.Annotations["zabbix_trigger_name"]; !ok {
						newTrigger.Description = v
					}
				case "zabbix_trigger_name":
					newTrigger.Description = v
				case "zabbix_trigger_description":
					newTrigger.Comments = v
				case "zabbix_trigger_severity":
					newTrigger.Priority = GetZabbixPriority(v)
				default:
					continue
				}
			}

			// If no applications are found in the rule, add the default application declared in the configuration
			if len(newItem.Applications) == 0 {
				newHost.AddApplication(&CustomApplication{
					State: StateNew,
					Application: zabbix.Application{
						Name: hostConfig.ItemDefaultApplication,
					},
				})
				newItem.Applications[hostConfig.ItemDefaultApplication] = struct{}{}
			}

			log.Debugf("Item from Prometheus: %+v", newItem)
			newHost.AddItem(newItem)

			log.Debugf("Trigger from Prometheus: %+v", newTrigger)
			newHost.AddTrigger(newTrigger)

			// Add the special "No Data" trigger if requested
			if delay, ok := rule.Annotations["zabbix_trigger_nodata"]; ok {
				noDataTrigger := &CustomTrigger{
					State:   StateNew,
					Trigger: newTrigger.Trigger,
				}

				noDataTrigger.Trigger.Description = fmt.Sprintf("%s - no data for the last %s seconds", newTrigger.Trigger.Description, delay)
				noDataTrigger.Trigger.Expression = fmt.Sprintf("{%s:%s.nodata(%s)}", newHost.Name, key, delay)
				log.Debugf("Trigger from Prometheus: %+v", noDataTrigger)
				newHost.AddTrigger(noDataTrigger)
			}
		}
		log.Debugf("Host from Prometheus: %+v", newHost)
		p.AddHost(newHost)
	}
}

// Update created hosts with the current state in Zabbix
func (p *Provisioner) FillFromZabbix() {

	hostNames := make([]string, len(p.Config.ZabbixHosts))
	hostGroupNames := []string{}
	for i, _ := range p.Config.ZabbixHosts {
		hostNames[i] = p.Config.ZabbixHosts[i].Name
		hostGroupNames = append(hostGroupNames, p.Config.ZabbixHosts[i].HostGroups...)
	}

	// Getting Zabbix HostGroups
	zabbixHostGroups, err := p.Api.HostGroupsGet(zabbix.Params{
		"output": "extend",
		"filter": map[string][]string{
			"name": hostGroupNames,
		},
	})

	if err != nil {
		log.Fatal(err)
	}

	for _, zabbixHostGroup := range zabbixHostGroups {
		p.AddHostGroup(&CustomHostGroup{
			State:     StateOld,
			HostGroup: zabbixHostGroup,
		})
	}

	// Getting Zabbix Hosts
	zabbixHosts, err := p.Api.HostsGet(zabbix.Params{
		"output": "extend",
		"selectInventory": []string{
			"tag",
			"deployment_status",
		},
		"filter": map[string][]string{
			"host": hostNames,
		},
	})

	if err != nil {
		log.Fatal(err)
	}

	for _, zabbixHost := range zabbixHosts {

		// Getting Zabbix HostGroups
		zabbixHostGroups, err := p.Api.HostGroupsGet(zabbix.Params{
			"output":  "extend",
			"hostids": zabbixHost.HostId,
		})

		if err != nil {
			log.Fatal(err)
		}

		hostGroups := make(map[string]struct{}, len(zabbixHostGroups))
		for _, zabbixHostGroup := range zabbixHostGroups {
			hostGroups[zabbixHostGroup.Name] = struct{}{}
		}

		// Remove hostid because the Zabbix API add it automatically and it breaks the comparison
		// between new/old hosts
		delete(zabbixHost.Inventory, "hostid")

		oldHost := p.AddHost(&CustomHost{
			State:        StateOld,
			Host:         zabbixHost,
			HostGroups:   hostGroups,
			Items:        map[string]*CustomItem{},
			Applications: map[string]*CustomApplication{},
			Triggers:     map[string]*CustomTrigger{},
		})
		log.Debugf("Host from Zabbix: %+v", oldHost)

		// Getting host applications
		zabbixApplications, err := p.Api.ApplicationsGet(zabbix.Params{
			"output":  "extend",
			"hostids": oldHost.HostId,
		})

		if err != nil {
			log.Fatal(err)
		}

		for _, zabbixApplication := range zabbixApplications {
			oldHost.AddApplication(&CustomApplication{
				State:       StateOld,
				Application: zabbixApplication,
			})
		}

		// Getting Zabbix Items
		zabbixItems, err := p.Api.ItemsGet(zabbix.Params{
			"output":  "extend",
			"hostids": oldHost.Host.HostId,
		})

		if err != nil {
			log.Fatal(err)
		}

		for _, zabbixItem := range zabbixItems {

			newItem := &CustomItem{
				State: StateOld,
				Item:  zabbixItem,
			}

			// Getting applications linkd to that item
			zabbixApplications, err := p.Api.ApplicationsGet(zabbix.Params{
				"output":  "extend",
				"itemids": zabbixItem.ItemId,
			})

			if err != nil {
				log.Fatal(err)
			}

			newItem.Applications = make(map[string]struct{}, len(zabbixApplications))
			for _, zabbixApplication := range zabbixApplications {
				newItem.Applications[zabbixApplication.Name] = struct{}{}
			}

			log.Debugf("Item from Zabbix: %+v", newItem)
			oldHost.AddItem(newItem)
		}

		// Get all the triggers for that host
		zabbixTriggers, err := p.Api.TriggersGet(zabbix.Params{
			"output":           "extend",
			"hostids":          oldHost.Host.HostId,
			"expandExpression": true,
		})

		if err != nil {
			log.Fatal(err)
		}

		for _, zabbixTrigger := range zabbixTriggers {
			newTrigger := &CustomTrigger{
				State:   StateOld,
				Trigger: zabbixTrigger,
			}

			log.Debugf("Triggers from Zabbix: %+v", newTrigger)
			oldHost.AddTrigger(newTrigger)
		}
	}
}

func (p *Provisioner) ApplyChanges() {

	hostGroupsByState := p.GetHostGroupsByState()
	if len(hostGroupsByState[StateNew]) != 0 {
		log.Debugf("Creating HostGroups: %+v\n", hostGroupsByState[StateNew])
		err := p.Api.HostGroupsCreate(hostGroupsByState[StateNew])
		if err != nil {
			log.Fatalln("Creating hostgroups:", err)
		}
	}

	// Make sure we update ids for the newly created host groups
	p.PropagateCreatedHostGroups(hostGroupsByState[StateNew])

	hostsByState := p.GetHostsByState()
	if len(hostsByState[StateNew]) != 0 {
		log.Debugf("Creating Hosts: %+v\n", hostsByState[StateNew])
		err := p.Api.HostsCreate(hostsByState[StateNew])
		if err != nil {
			log.Fatalln("Creating host:", err)
		}
	}

	// Make sure we update ids for the newly created hosts
	p.PropagateCreatedHosts(hostsByState[StateNew])

	if len(hostsByState[StateUpdated]) != 0 {
		log.Debugf("Updating Hosts: %+v\n", hostsByState[StateUpdated])
		err := p.Api.HostsUpdate(hostsByState[StateUpdated])
		if err != nil {
			log.Fatalln("Updating host:", err)
		}
	}

	//if len(hostsByState[StateOld]) != 0 {
	//	log.Debugf("Deleting Hosts: %+v\n", hostsByState[StateOld])
	//	err := p.Api.HostsDelete(hostsByState[StateOld])
	//	if err != nil {
	//		log.Fatalln("Deleting host:", err)
	//	}
	//}

	//if len(hostGroupsByState[StateOld]) != 0 {
	//	log.Debugf("Deleting HostGroups: %+v\n", hostGroupsByState[StateOld])
	//	err := p.Api.HostGroupsDelete(hostGroupsByState[StateOld])
	//	if err != nil {
	//		log.Fatalln("Deleting hostgroups:", err)
	//	}
	//}

	for _, host := range p.Hosts {

		log.Infoln("Updating host:", host.Name)

		applicationsByState := host.GetApplicationsByState()
		if len(applicationsByState[StateOld]) != 0 {
			log.Debugf("Deleting applications: %+v\n", applicationsByState[StateOld])
			err := p.Api.ApplicationsDelete(applicationsByState[StateOld])
			if err != nil {
				log.Fatalln("Deleting applications:", err)
			}
		}

		if len(applicationsByState[StateNew]) != 0 {
			log.Debugf("Creating applications: %+v\n", applicationsByState[StateNew])
			err := p.Api.ApplicationsCreate(applicationsByState[StateNew])
			if err != nil {
				log.Fatalln("Creating applications:", err)
			}
		}
		host.PropagateCreatedApplications(applicationsByState[StateNew])

		itemsByState := host.GetItemsByState()
		triggersByState := host.GetTriggersByState()

		if len(triggersByState[StateOld]) != 0 {
			log.Debugf("Deleting triggers: %+v\n", triggersByState[StateOld])
			err := p.Api.TriggersDelete(triggersByState[StateOld])
			if err != nil {
				log.Fatalln("Deleting triggers:", err)
			}
		}

		if len(itemsByState[StateOld]) != 0 {
			log.Debugf("Deleting items: %+v\n", itemsByState[StateOld])
			err := p.Api.ItemsDelete(itemsByState[StateOld])
			if err != nil {
				log.Fatalln("Deleting items:", err)
			}
		}

		if len(itemsByState[StateUpdated]) != 0 {
			log.Debugf("Updating items: %+v\n", itemsByState[StateUpdated])
			err := p.Api.ItemsUpdate(itemsByState[StateUpdated])
			if err != nil {
				log.Fatalln("Updating items:", err)
			}
		}

		if len(triggersByState[StateUpdated]) != 0 {
			log.Debugf("Updating triggers: %+v\n", triggersByState[StateUpdated])
			err := p.Api.TriggersUpdate(triggersByState[StateUpdated])
			if err != nil {
				log.Fatalln("Updating triggers:", err)
			}
		}

		if len(itemsByState[StateNew]) != 0 {
			log.Debugf("Creating items: %+v\n", itemsByState[StateNew])
			err := p.Api.ItemsCreate(itemsByState[StateNew])
			if err != nil {
				log.Fatalln("Creating items:", err)
			}
		}

		if len(triggersByState[StateNew]) != 0 {
			log.Debugf("Creating triggers: %+v\n", triggersByState[StateNew])
			err := p.Api.TriggersCreate(triggersByState[StateNew])
			if err != nil {
				log.Fatalln("Creating triggers:", err)
			}
		}
	}

}
