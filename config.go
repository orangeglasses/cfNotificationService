package main

import (
	"io/ioutil"
	"log"

	"github.com/cloudfoundry-community/go-cfenv"
	"github.com/kelseyhightower/envconfig"
)

type notificationServerConfig struct {
	CFApi      map[string]string `envconfig:"cf_api" required:"true"`
	CFUser     map[string]string `envconfig:"cf_user"`
	CFPassword map[string]string `envconfig:"cf_password"`
	CFClient   map[string]string `envconfig:"cf_client"`
	CFSecret   map[string]string `envconfig:"cf_secret"`

	EmailHost     string `envconfig:"email_host" required:"true"`
	EmailPort     int    `envconfig:"email_port" required:"true"`
	EmailFrom     string `envconfig:"email_from" required:"true"`
	EmailUser     string `envconfig:"email_user" required:"false"`
	EmailPassword string `envconfig:"email_password" required:"false"`

	IpaHost     string `envconfig:"ipa_host" required:"false"`
	IpaUser     string `envconfig:"ipa_user" required:"false"`
	IpaPassword string `envconfig:"ipa_password" required:"false"`

	RedisHost     string `envconfig:"redis_host" required:"false"`
	RedisPort     int    `envconfig:"redis_port" default:"6379"`
	RedisPassword string `envconfig:"redis_password" default:""`
	RedisDB       int    `envconfig:"redis_db" default:"0"`

	RabbitURI           string            `envconfig:"rabbit_uri" required:"false"`
	RabbitExchange      string            `envconfig:"rabbit_exchange" required:"false"`
	RabbitTemplateFiles map[string]string `envconfig:"rabbit_template_files" required:"false"`
	//RabbitRoutingKeys   []string          `envconfig:"rabbit_routinkeys" required:"false"`
	RabbitTemplates map[string]string

	ApiUsers map[string]string `envconfig:"api_users" required:"true"`

	ClientID         string `envconfig:"CLIENT_ID" required:"true"`
	ClientSecret     string `envconfig:"CLIENT_SECRET" required:"true"`
	OauthProviderUrl string `envconfig:"OAUTH_PROVIDER_URL" required:"true"` //"https://uaa.sys.cf.automate-it.lab/oauth/token"
	SessionKey       string `envconfig:"SESSION_KEY" required:"true"`

	AppName string `envconfig:"app_name"`
	AppInfo string `envconfig:"app_info"`

	WelcomeSubject string `envconfig:"welcome_subject" required:"true"`
	WelcomeMessage string `envconfig:"welcome_message" required:"true"`
	GoodbyeSubject string `envconfig:"goodbye_subject" required:"true"`
	GoodbyeMessage string `envconfig:"goodbye_message" required:"true"`

	AppPort int `default:"9000"`
}

func notificationServerConfigLoad() (notificationServerConfig, error) {
	var config notificationServerConfig
	err := envconfig.Process("", &config)
	if err != nil {
		return notificationServerConfig{}, err
	}

	if len(config.CFUser) == 0 && len(config.CFClient) == 0 {
		log.Fatal("Please set CF_USER/CF_PASSWORD or CF_CLIENT/CF_SECRET")
	}

	if len(config.CFUser) != 0 && len(config.CFClient) != 0 {
		log.Println("Both CF_USER and CF_CLIENT are set. I'll use CF_CLIENT and ignore CF_USER.")
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

	if config.IpaHost != "" && (config.IpaUser == "" || config.IpaPassword == "") {
		log.Fatalln("IPA host configured but username or password are empty.")
	}

	if config.RabbitURI != "" {
		log.Println("Rabbit configured, loading templates files.")
		config.RabbitTemplates = make(map[string]string)
		for providerName, filePath := range config.RabbitTemplateFiles {
			log.Println("  ", filePath)
			inBuf, err := ioutil.ReadFile(filePath)
			if err != nil {
				return notificationServerConfig{}, err
			}
			config.RabbitTemplates[providerName] = string(inBuf)
		}
	}

	return config, nil
}
