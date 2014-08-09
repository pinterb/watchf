package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"strings"
	"time"
)

var (
	defaultConfig = &Config{Version: Version, Events: []string{"all"}, Commands: []string{}}
)

// Config models the configuration for watchf
type Config struct {
	Recursive      bool
	Events         CommaStringSet
	IncludePattern string
	Commands       StringSet
	Interval       time.Duration
	Version        string
}

// StringSet is a simple string array
type StringSet []string

// CommaStringSet is a string array (for comma delimited strings)
type CommaStringSet []string

func init() {
	flag.BoolVar(&defaultConfig.Recursive, "r", false, "Watch directories recursively")
	flag.StringVar(&defaultConfig.IncludePattern, "p", ".*", "File name matches regular expression pattern (perl-style)")
	flag.DurationVar(&defaultConfig.Interval, "i", time.Duration(0)*time.Millisecond, "The interval limit the frequency of the command executions, if equal to 0, there is no limit (time unit: ns/us/ms/s/m/h)")
	flag.Var(&defaultConfig.Events, "e", "Listen for specific event(s) (comma separated list)")
	flag.Var(&defaultConfig.Commands, "c", "Add arbitrary command (repeatable)")
}

// GetDefaultConfig returns a pointer to default configuration
func GetDefaultConfig() *Config {
	return defaultConfig
}

// WriteConfigToFile will persist a Config
func WriteConfigToFile(config *Config) (err error) {
	rawdata, err := json.MarshalIndent(&config, "", "	")
	if err != nil {
		return
	}
	err = ioutil.WriteFile(configFile, rawdata, 0644)
	return
}

// LoadConfigFromFile creates a Config from a persisted configuration file
func LoadConfigFromFile() (newConfig *Config, err error) {
	// TODO: check compatibility
	newConfig = &Config{}
	rawdata, err := ioutil.ReadFile(configFile)
	if err != nil {
		return
	}
	err = json.Unmarshal(rawdata, newConfig)
	return
}

// String formats StringSet
func (f *StringSet) String() string {
	return fmt.Sprint([]string(*f))
}

// Set will append a string value to a StringSet
func (f *StringSet) Set(value string) error {
	*f = append(*f, value)
	return nil
}

// String formats CommaStringSet
func (f *CommaStringSet) String() string {
	return fmt.Sprint([]string(*f))
}

// Set will parse a comma delimited string into a CommaStringSet
func (f *CommaStringSet) Set(value string) error {
	*f = strings.Split(strings.Replace(value, " ", "", -1), ",")
	return nil
}
