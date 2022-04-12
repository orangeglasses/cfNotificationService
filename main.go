package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/cloudfoundry-community/go-cfenv"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
)

func main() {
	appEnv, _ := cfenv.Current()
	config, err := notificationServerConfigLoad()
	if err != nil {
		log.Fatalf("Error loading config: %v", err.Error())
	}

	cfSpaceUserGetter := NewCfSpaceUserGetter()

	for environment, cfapi := range config.CFApi {

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
			log.Fatal("Failed logging into cloudfoundry", err)
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
		apiUsers:     config.ApiUsers,
		sessionStore: sessions.NewCookieStore([]byte(config.SessionKey)),
		appName:      config.AppName,
	}

	ns.RegisterUserGetter("space", cfSpaceUserGetter)
	ns.RegisterNotificationSender("email", NewEmailSender(config.EmailHost, config.EmailPort, config.EmailFrom))

	r := mux.NewRouter()
	r.Path("/").Methods(http.MethodGet).HandlerFunc(ns.rootHandler)
	r.Path("/send").Methods(http.MethodPost).HandlerFunc(ns.sendHandler)

	r.Path("/subscribe/{username}").Methods(http.MethodPost).HandlerFunc(ns.subscribeHandler)

	r.Path("/logout").HandlerFunc(ns.HandleLogout)
	r.Path("/login").HandlerFunc(ns.HandleRedirect)
	r.Path("/oauth2").HandlerFunc(ns.HandleOauthCallback)

	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	http.ListenAndServe(fmt.Sprintf(":%v", appEnv.Port), r)
}
