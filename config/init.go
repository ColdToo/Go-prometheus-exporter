package config

import (
	"encoding/json"
	"etcdvalue_exporter/etcd"
	"etcdvalue_exporter/log"
	"etcdvalue_exporter/mysql"
	"etcdvalue_exporter/redis"
	"fmt"
	"io/ioutil"
	"os"
)

const (
	// ETCDConfNotExistMsg is error message if etcd config file not exist
	ETCDConfNotExistMsg = "etcd config file not exist"
	// ETCDConfCheckInterval is the interval seconds before next check for etcd config file
	ETCDConfCheckInterval = 60
	ErrMysqlInit          = "bgorm.Init() err"
	DefaultRceAgentPort   = 5051
)

var gConfig *Config
var configSet bool

// Initialize initializes configuration with given config file path
func Initialize(path string) (*Config, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file error: %v", err)
	}

	config := Config{}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("config initialized error: %v", err)
	}

	if config.RceAgent.Port == 0 {
		config.RceAgent.Port = DefaultRceAgentPort
	}

	// initialize log configuration
	if err := log.Initialize(config.Log); err != nil {
		return nil, fmt.Errorf("log config initialized error: %v", err)
	}

	if err := etcd.Initialize(config.ETCDConfPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf(ETCDConfNotExistMsg)
		}
		return nil, fmt.Errorf("etcd config initialized error: %v", err)
	}

	if err := mysql.InitMySQL(config.MysqlDb); err != nil {
		return nil, fmt.Errorf("mysql config initialized error: %v", err)
	}

	if err := redis.InitRedis(config.Redis); err != nil {
		return nil, fmt.Errorf("redis config initialized  failed with :%v", err)
	}

	gConfig = &config
	configSet = true
	return gConfig, nil
}

func GetConfig() *Config {
	if configSet {
		return gConfig
	}
	return nil
}
