package config

import (
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

type Conf struct {
	SubmitReqUrl  string   `yaml:"submit_req_url"`
	ProblemReqUrl string   `yaml:"problem_req_url"`
	UserName      []string `yaml:"user_name"`
	WebhookUrl    string   `yaml:"webhook_url"`
	DbPath        string   `yaml:"db_path"`
}

func readConf(yaml_path string) (Conf, error) {
	buf, err := ioutil.ReadFile(yaml_path)
	p := Conf{}
	if err != nil {
		return p, err
	}
	yaml.Unmarshal(buf, &p)
	return p, err
}
