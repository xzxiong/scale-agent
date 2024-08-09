// Copyright 2024 Matrix Origin.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"os"
	"strings"

	"github.com/go-logr/logr"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/labels"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var gCfg *Configuration

func InitConfiguration(l logr.Logger, cfgPath string) (*Configuration, error) {
	var cfg = NewConfiguration()
	viper.SetConfigFile(cfgPath)
	if err := viper.ReadInConfig(); err != nil {
		l.Error(err, "failed to read config file")
		return nil, err
	}
	for _, key := range viper.AllKeys() {
		if value := os.Getenv(key); value != "" {
			viper.Set(key, value)
		}
	}
	if err := viper.Unmarshal(cfg); err != nil {
		l.Error(err, "failed to unmarshal config file")
		return nil, err
	}
	setConfiguration(cfg)
	return cfg, nil
}

type Configuration struct {
	App AppConfig `yaml:"app"`
	MO  MOConfig  `yaml:"mo"`
}

func (c *Configuration) Validate() error {
	if _, err := c.MO.GetLabelSelector(); err != nil {
		return err
	}
	return nil
}

func GetConfiguration() *Configuration {
	if gCfg == nil {
		gCfg = NewConfiguration()
	}
	return gCfg
}

// setConfiguration should only do ONCE in main.go
func setConfiguration(cfg *Configuration) {
	gCfg = cfg
}

func NewConfiguration() *Configuration {
	return &Configuration{
		App: *NewAppConfig(),
		MO:  *NewMOConfig(),
	}
}

type AppConfig struct {
	NodeName      string `yaml:"nodeName"`
	ResourceRange string `yaml:"resourceRange"`
}

func NewAppConfig() *AppConfig {
	return &AppConfig{
		ResourceRange: KeyMatrixoneCloudResourceRange,
	}
}

func (c *AppConfig) GetNodeName() string {
	return c.NodeName
}

func (c *AppConfig) SetNodeName(name string) {
	c.NodeName = name
}

type MOConfig struct {
	Labels []string `yaml:"labels"`

	parsedLabels map[string]string
}

func NewMOConfig() *MOConfig {
	return &MOConfig{}
}

func (c *MOConfig) GetLabelSelector() (labels.Selector, error) {
	if len(c.parsedLabels) == 0 {
		c.parsedLabels = ParseLabelsAsMap(c.Labels)
	}
	return metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: c.parsedLabels,
	})
}

// ParseLabelsAsMap split each elem in labels by '=' as map's key and value
func ParseLabelsAsMap(labels []string) map[string]string {
	m := make(map[string]string, len(labels))
	for _, e := range labels {
		if !strings.Contains(e, "=") {
			m[e] = ""
		} else {
			kv := strings.Split(e, "=")
			m[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return m
}
