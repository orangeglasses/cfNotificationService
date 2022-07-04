package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"

	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/cloudfoundry-community/go-cfenv"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/vchrisr/go-freeipa/freeipa"
	"golang.org/x/oauth2"
)

func main() {
	appEnv, _ := cfenv.Current()
	config, err := notificationServerConfigLoad()
	if err != nil {
		log.Fatalf("Error loading config: %v", err.Error())
	}

	cfSpaceUserGetter := NewCfSpaceUserGetter()

	log.Println("Loading environments...")
	for environment, cfapi := range config.CFApi {
		log.Println("Creating CF client for ", environment)
		c := &cfclient.Config{
			ApiAddress:        "https://" + cfapi,
			SkipSslValidation: false,
		}

		if cfuser, ok := config.CFUser[environment]; ok {
			c.Username = cfuser
			c.Password = config.CFPassword[environment]
		} else {
			c.ClientID = config.CFClient[environment]
			c.ClientSecret = config.CFSecret[environment]
		}

		cfClient, err := cfclient.NewClient(c)
		if err != nil {
			log.Fatalln("Failed logging into cloudfoundry", err)
		}

		cfSpaceUserGetter.RegisterEnvironment(environment, cfClient)
	}

	provider, err := oidc.NewProvider(context.Background(), config.OauthProviderUrl)
	if err != nil {
		log.Fatal(err)
	}

	ns := &notificationServer{
		oauthConfig: oauth2.Config{
			ClientID:     config.ClientID,
			ClientSecret: config.ClientSecret,
			Endpoint:     provider.Endpoint(),
			RedirectURL:  "https://" + appEnv.ApplicationURIs[0] + "/oauth2",
			Scopes:       []string{oidc.ScopeOpenID},
		},
		oidcProvider: provider,
		oidcConfig: &oidc.Config{
			ClientID: config.ClientID,
		},
		redisClient: redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%v:%v", config.RedisHost, config.RedisPort),
			Password: config.RedisPassword,
			DB:       config.RedisDB,
		}),
		apiUsers:       config.ApiUsers,
		sessionStore:   sessions.NewCookieStore([]byte(config.SessionKey)),
		appName:        config.AppName,
		appInfo:        config.AppInfo,
		welcomeSubject: config.WelcomeSubject,
		welcomeMessage: config.WelcomeMessage,
		goodbyeSubject: config.GoodbyeSubject,
		goodbyeMessage: config.GoodbyeMessage,
	}

	ns.RegisterUserGetter("space", cfSpaceUserGetter)

	if config.IpaHost != "" {
		ipaClient, err := freeipa.Connect(config.IpaHost, &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}, config.IpaUser, config.IpaPassword)

		if err != nil {
			log.Fatal(err)
		}

		ns.RegisterUserGetter("idmgroup", NewIpaUserGetter(ipaClient))
	}

	ns.RegisterNotificationSender("email", NewEmailSender(config.EmailHost, config.EmailPort, config.EmailFrom))

	if config.RabbitURI != "" {
		for rabbitSender, template := range config.RabbitTemplates {
			log.Printf("Creating rabbitSender %s. Using exchange: %s\n", rabbitSender, config.RabbitExchange)
			ns.RegisterNotificationSender(rabbitSender, NewRabbitSender(config.RabbitURI, config.RabbitExchange, template))
		}
	}

	collector := NewStatsCollector(ns.redisClient)
	prometheus.MustRegister(collector)

	r := mux.NewRouter()
	r.Path("/").Methods(http.MethodGet).HandlerFunc(ns.rootHandler)
	r.Path("/send").Methods(http.MethodPost).HandlerFunc(ns.sendHandler)

	r.Path("/subscribe/{username}").Methods(http.MethodPost).HandlerFunc(ns.subscribeHandler)

	r.Path("/logout").HandlerFunc(ns.HandleLogout)
	r.Path("/login").HandlerFunc(ns.HandleRedirect)
	r.Path("/oauth2").HandlerFunc(ns.HandleOauthCallback)

	r.Path("/stats").HandlerFunc(collector.statsHandler)
	r.Path("/metrics").Handler(promhttp.Handler())

	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	http.ListenAndServe(fmt.Sprintf(":%v", appEnv.Port), r)
}
