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
	NodeName      string `yaml:"node-name"`
	ResourceRange string `yaml:"resource-range"`
}

func NewAppConfig() *AppConfig {
	return &AppConfig{
		ResourceRange: KeyMatrixoneCloudResourceRange,
	}
}

func (c *AppConfig) GetNodeName() string {
	if c.NodeName == NodeNameAuto {
		c.NodeName = os.Getenv(EnvPodName)
	}
	return c.NodeName
}

type MOConfig struct {
	Labels map[string]string `yaml:"labels"`
}

func NewMOConfig() *MOConfig {
	return &MOConfig{
		Labels: make(map[string]string),
	}
}

func (c *MOConfig) GetLabelSelector() (labels.Selector, error) {
	return metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: c.Labels,
	})
}
