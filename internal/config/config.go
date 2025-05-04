package config

/*
In setting up config, we will have 3 types of config:

	* file cofig.json: contains all basic config information such as the servers, the log level, and the environment
	* file .env: contains all secret information
	* file config.go: contains all information including but not limited to basic config, database config, and other various config needed
*/

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

type jsonConfig struct {
	Servers map[string]struct {
		Url    string `json:"url"`
		Name   string `json:"name"`
		Assets []struct {
			AssetName string `json:"asset_name"`
		} `json:"assets"`
		Channels []struct {
			ChannelName string `json:"channel_name"`
		} `json:"channels"`
	} `json:"servers"`
	LogLevel    string `json:"log_level,omitempty"`
	Environment string `json:"environment,omitempty"`
}

type Config struct {
	Databases map[string]*DatabaseConfig
	Servers   map[string]*ServerConfig
}

type DatabaseConfig struct {
	DbType   string //defining the type of the database
	Host     string // defining the host of the database
	Port     string // defining the port used for the database
	User     string // defining the user of the database
	Password string // defining the password to accesss the database
	DbName   string // defining the database name to be accessed
	SslMode  string // defining the sslmode applied to the database
	Path     string // defining the path used to access the database
}

type ServerConfig struct {
	Name     string            // the name of the server that we are accessing
	Url      string            // the url of the server we are accessing
	Headers  map[string]string // the header of the content
	Assets   []Asset           // list of asset that we are going to trade
	Channels []Channel         // list of channels we are listening to
}

type Asset struct {
	Name string
}

type Channel struct {
	Name string
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)

	if value == "" {
		return defaultValue
	}
	return value
}

func InitConfig() (*Config, error) {
	// load the environemnt to expose the contents
	_ = godotenv.Load()

	postgres_ := &DatabaseConfig{
		DbType:   "postgres",
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnv("DB_PORT", "5432"),
		User:     getEnv("DB_USERNAME", "postgres"),
		Password: getEnv("DB_PASSWORD", "pass"),
		DbName:   getEnv("DB_NAME", "database"),
		SslMode:  getEnv("DB_SSLMODE", "disable"),
	}

	sqlite3_ := &DatabaseConfig{
		DbType: "sqlite3",
		Path:   getEnv("SQLITE_PATH_NAME", ""),
	}

	// now let's read the json to expose our base configuration
	file, err := os.ReadFile("config.json")
	if err != nil {
		return nil, fmt.Errorf("error reading json config file: %v", err)
	}

	var rawConfig jsonConfig
	err = json.Unmarshal(file, &rawConfig)
	if err != nil {
		return nil, fmt.Errorf("error unmarshing json config: %v", err)
	}

	config := make(map[string]*ServerConfig)

	for name, value := range rawConfig.Servers {
		server := &ServerConfig{
			Name:     name,
			Url:      value.Url,
			Assets:   make([]Asset, len(value.Assets)),
			Channels: make([]Channel, len(value.Channels)),
		}

		for i, rawAsset := range value.Assets {
			server.Assets[i] = Asset{Name: rawAsset.AssetName}
		}

		for i, rawChannel := range value.Channels {
			server.Channels[i] = Channel{Name: rawChannel.ChannelName}
		}

		config[name] = server
	}

	log.Println("[config.go] Configuration loaded!")

	return &Config{
		Databases: map[string]*DatabaseConfig{
			"postgres": postgres_,
			"sqlite3":  sqlite3_,
		},
		Servers: config,
	}, nil
}
