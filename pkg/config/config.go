package config

import (
)


type Config struct {
	Listen []string       `yaml:"listen"`
	Peers map[string]Peer `yaml:"peers"`
}

type Peer struct {
	Name				string `yaml:"name"`
	Port     			int16  `yaml:"port"`
	Interval 			int    `yaml:"interval"`  			// target interval in ms
	DetectionMultiplier int    `yaml:"detectionMultiplier"`
}