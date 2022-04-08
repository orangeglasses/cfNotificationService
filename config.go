package main

import (
	"log"

	"github.com/cloudfoundry-community/go-cfenv"
	"github.com/kelseyhightower/envconfig"
)

type notificationServerConfig struct {
	CFApi      map[string]string `envconfig:"cf_api" required:"true"`
	CFUser     map[string]string `envconfig:"cf_user" required:"true"`
	CFPassword map[string]string `envconfig:"cf_password" required:"true"`

	EmailHost     string `envconfig:"email_host" required:"true"`
	EmailPort     int    `envconfig:"email_port" required:"true"`
	EmailFrom     string `envconfig:"email_from" required:"true"`
	EmailUser     string `envconfig:"email_user" required:"false"`
	EmailPassword string `envconfig:"email_password" required:"false"`

	RedisHost     string `envconfig:"redis_host" required:"false"`
	RedisPort     int    `envconfig:"redis_port" default:"6379"`
	RedisPassword string `envconfig:"redis_password" default:""`
	RedisDB       int    `envconfig:"redis_db" default:"0"`

	ApiUsers map[string]string `envconfig:"api_users" required:"true"`

	ClientID         string `envconfig:"CLIENT_ID" required:"true"`
	ClientSecret     string `envconfig:"CLIENT_SECRET" required:"true"`
	OauthProviderUrl string `envconfig:"OAUTH_PROVIDER_URL" required:"true"` //"https://uaa.sys.cf.automate-it.lab/oauth/token"
	SessionKey       string `envconfig:"SESSION_KEY" required:"true"`

	AppName string `envconfig:"app_name"`

	AppPort int `default:"9000"`
}

func notificationServerConfigLoad() (notificationServerConfig, error) {
	var config notificationServerConfig
	err := envconfig.Process("", &config)
	if err != nil {
		return notificationServerConfig{}, err
	}

	if cfenv.IsRunningOnCF() {
		appEnv, _ := cfenv.Current()
		config.AppPort = appEnv.Port

		redisServices, err := appEnv.Services.WithTag("redis")
		if err != nil {
			log.Fatal("No Redis service bound to this app", err)
		}
		config.RedisHost = redisServices[0].Credentials["host"].(string)
		config.RedisPort = int(redisServices[0].Credentials["port"].(float64))
		config.RedisPassword = redisServices[0].Credentials["password"].(string)
	} else {
		if config.RedisHost == "" {
			log.Fatalln("No Redis host configured. Please set REDIS_HOST env var.")
		}
	}

	return config, nil
}
