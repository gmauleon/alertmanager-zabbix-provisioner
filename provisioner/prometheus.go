package provisioner

import (
	"encoding/json"
	"golang.org/x/net/html"
	"io/ioutil"
	"net/http"
	"strings"
)

type PrometheusRule struct {
	Name        string            `json:"name"`
	Annotations map[string]string `json:"annotations"`
}

type PrometheusResponse struct {
	Rules []PrometheusRule `json:"rules"`
}

func GetRulesFromJSON() []PrometheusRule {
	rulesFile, err := ioutil.ReadFile("rules2.json")
	if err != nil {
		log.Fatalf("Can't open the rules file: %s", err)
	}

	response := PrometheusResponse{}
	err = json.Unmarshal(rulesFile, &response)
	if err != nil {
		log.Fatalf("Can't read the rules file: %s", err)
	}

	return response.Rules
}

func GetRulesFromURL(url string) []PrometheusRule {
	resp, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}

	tokenizer := html.NewTokenizer(resp.Body)

	var key string
	var rule PrometheusRule
	rules := []PrometheusRule{}

	for {
		tokenType := tokenizer.Next()
		switch tokenType {
		case html.ErrorToken:
			//log.Info(rules)
			return rules
		case html.TextToken:
			str := string(tokenizer.Text())
			if strings.HasPrefix(str, "ALERT") {
				tokenizer.Next()
				tokenizer.Next()
				rule = PrometheusRule{
					Name:        string(tokenizer.Text()),
					Annotations: map[string]string{},
				}
				rules = append(rules, rule)
				//log.Infof("Rule: %s", string(tokenizer.Text()))
			} else if strings.Contains(str, "ANNOTATIONS") {
				//log.Info(str)
				raw := strings.SplitAfter(str, "ANNOTATIONS")
				splits := strings.Split(raw[1], "\"")
				//log.Info(splits)
				for index, split := range splits {
					trimmed := strings.Trim(split, " {}")
					if len(trimmed) != 0 {
						if index%2 == 0 {
							replacer := strings.NewReplacer("=", "", " ", "", ",", "")
							//log.Printf("Key: %s", replacer.Replace(trimmed))
							key = replacer.Replace(trimmed)
						} else {
							//log.Printf("Value: %s", trimmed)
							rule.Annotations[key] = trimmed
						}
					}
				}
			}
		}
	}
}
