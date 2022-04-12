package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/mail"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
)

type notificationServer struct {
	redisClient         *redis.Client
	userGetters         UserGetters
	notificationSenders NotificationSenders
	apiUsers            map[string]string
	sessionStore        *sessions.CookieStore
	oauthConfig         oauth2.Config
	oidcProvider        *oidc.Provider
	oidcConfig          *oidc.Config
	appName             string
}

type UserGetter interface {
	Get(string, string) ([]string, error)
}

type NotificationSender interface {
	Send(string, string, string) error
}

type UserGetters map[string]UserGetter
type NotificationSenders map[string]NotificationSender

type Subscription struct {
	Addresses map[string]string `json:"addresses"`
}

func (s Subscription) MarshalBinary() ([]byte, error) {
	return json.Marshal(s)
}

func (ns *notificationServer) RegisterUserGetter(name string, ug UserGetter) {
	if ns.userGetters == nil {
		ns.userGetters = make(UserGetters)
	}
	ns.userGetters[name] = ug
}

func (ns *notificationServer) RegisterNotificationSender(name string, sender NotificationSender) {
	if ns.notificationSenders == nil {
		ns.notificationSenders = make(NotificationSenders)
	}
	ns.notificationSenders[name] = sender
}

func (ns *notificationServer) sendHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	u, p, ok := r.BasicAuth()
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if expectedPw, ok := ns.apiUsers[u]; !ok || expectedPw != p {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	ctx := r.Context()

	var msg messageBody

	err := json.NewDecoder(r.Body).Decode(&msg)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	exp, err := time.ParseDuration(msg.ExpiresIn)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	//check if we sent message with sme id previously
	_, err = ns.redisClient.Get(ctx, msg.Id).Result()
	if err == nil { //message found, don't sent it again
		w.WriteHeader(http.StatusConflict)
		fmt.Fprintf(w, "existing message, not sending again")
		return
	}

	//message not stored, let's store it if it has expiration set
	if exp > 0 {
		_, err = ns.redisClient.Set(ctx, msg.Id, msg, exp).Result()
		if err != nil {
			w.WriteHeader(http.StatusAccepted)
			fmt.Fprintf(w, "message accepted but unable to store")
			log.Println(err)
		}
	}

	//retrieve recipient user names based on target
	var users []string
	if getUsers, ok := ns.userGetters[msg.Target.Type]; ok {
		users, err = getUsers.Get(msg.Target.Environment, msg.Target.Id)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Error retrieving users: %v", err.Error())
			return
		}
	} else {
		w.WriteHeader(http.StatusNotImplemented)
		fmt.Fprintf(w, "%s target type not implemented yet\n", msg.Target.Type)
		return
	}

	//get destination adress/number for each user from redis
	subScriptions := make(map[string]Subscription)
	for _, u := range users {
		fmt.Printf("Finding contact info for %v\n", u)
		ciString, err := ns.redisClient.Get(ctx, u).Result()
		if err != nil {
			fmt.Println("No info found")
			continue
		}

		var ci Subscription
		err = json.Unmarshal([]byte(ciString), &ci)
		if err == nil {
			subScriptions[u] = ci
		}
	}

	//check if there are any recipients
	if len(subScriptions) == 0 {
		log.Println("Message send to target without recipients")
		fmt.Fprintf(w, "No recipients found for %s with id %s", msg.Target.Type, msg.Id)
	}

	//and then sent it
	for _, ci := range subScriptions {
		for addressType, address := range ci.Addresses {
			if sender, ok := ns.notificationSenders[addressType]; ok {
				go sender.Send(address, msg.Subject, msg.Message)
			} else {
				log.Printf("Address type %s not valid\n", addressType)
			}

			fmt.Fprintf(w, "message sent")
		}
	}
}

func (ns *notificationServer) subscribeHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	vars := mux.Vars(r)
	username, _ := vars["username"]

	session, _ := ns.sessionStore.Get(r, "sub-session")
	if u, ok := session.Values["userName"]; !ok || u != username {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	address := r.PostFormValue("address")
	subType := r.PostFormValue("type")

	//check if address type is valid
	if _, ok := ns.notificationSenders[subType]; !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	//delete record if email adres is empty
	if address == "" {
		ns.redisClient.Del(r.Context(), username).Result()
		http.Redirect(w, r, "//", http.StatusFound)
		return
	}

	if _, err := mail.ParseAddress(address); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, "Invalid email adres entered")
	}

	sub := Subscription{
		Addresses: make(map[string]string),
	}
	sub.Addresses[subType] = address

	_, err := ns.redisClient.Set(r.Context(), username, sub, 0).Result()
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusFound)
}

func (ns *notificationServer) rootHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	session, _ := ns.sessionStore.Get(r, "sub-session")
	username, ok := session.Values["userName"]
	if !ok || username == "" {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	subInfoString, _ := ns.redisClient.Get(r.Context(), username.(string)).Result()
	var subInfo Subscription
	if subInfoString != "" {
		json.Unmarshal([]byte(subInfoString), &subInfo)
	}

	tmpl := template.Must(template.ParseFiles("subscribe.tmpl"))

	data := struct {
		AppName    string
		Username   string
		Types      []string
		CurrentSub Subscription
		Subscribed bool
	}{
		AppName:    ns.appName,
		Username:   username.(string),
		Types:      ns.getSupportedSenders(),
		CurrentSub: subInfo,
		Subscribed: (len(subInfo.Addresses) > 0),
	}

	err := tmpl.Execute(w, data)
	if err != nil {
		log.Println(err)
	}
}

func (ns *notificationServer) HandleLogout(w http.ResponseWriter, r *http.Request) {
	session, _ := ns.sessionStore.Get(r, "sub-session")
	session.Values["userName"] = ""
	session.Save(r, w)
}

func (ns *notificationServer) HandleRedirect(w http.ResponseWriter, r *http.Request) {
	state, err := randString(16)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	nonce, err := randString(16)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	setCallbackCookie(w, r, "state", state)
	setCallbackCookie(w, r, "nonce", nonce)

	http.Redirect(w, r, ns.oauthConfig.AuthCodeURL(state, oidc.Nonce(nonce)), http.StatusFound)
}

func (ns *notificationServer) HandleOauthCallback(w http.ResponseWriter, r *http.Request) {
	state, err := r.Cookie("state")
	if err != nil {
		http.Error(w, "state not found", http.StatusBadRequest)
		return
	}
	if r.URL.Query().Get("state") != state.Value {
		http.Error(w, "state did not match", http.StatusBadRequest)
		return
	}

	oauth2Token, err := ns.oauthConfig.Exchange(r.Context(), r.URL.Query().Get("code"))
	if err != nil {
		http.Error(w, "Failed to exchange token: "+err.Error(), http.StatusInternalServerError)
		return
	}
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "No id_token field in oauth2 token.", http.StatusInternalServerError)
		return
	}

	verifier := ns.oidcProvider.Verifier(ns.oidcConfig)
	idToken, err := verifier.Verify(r.Context(), rawIDToken)
	if err != nil {
		http.Error(w, "Failed to verify ID Token: "+err.Error(), http.StatusInternalServerError)
		return
	}

	nonce, err := r.Cookie("nonce")
	if err != nil {
		http.Error(w, "nonce not found", http.StatusBadRequest)
		return
	}
	if idToken.Nonce != nonce.Value {
		http.Error(w, "nonce did not match", http.StatusBadRequest)
		return
	}

	var tokenClaim struct {
		UserId   string `json:"user_id"`
		UserName string `json:"user_name"`
	}

	if err := idToken.Claims(&tokenClaim); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	//create session
	session, _ := ns.sessionStore.Get(r, "sub-session")

	//store session
	session.Values["userName"] = tokenClaim.UserName
	session.Options.MaxAge = 60

	err = session.Save(r, w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusFound)
}

func randString(nByte int) (string, error) {
	b := make([]byte, nByte)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func setCallbackCookie(w http.ResponseWriter, r *http.Request, name, value string) {
	c := &http.Cookie{
		Name:     name,
		Value:    value,
		MaxAge:   int(time.Hour.Seconds()),
		Secure:   r.TLS != nil,
		HttpOnly: true,
	}
	http.SetCookie(w, c)
}

func (ns *notificationServer) getSupportedSenders() []string {
	var supportedTypes []string
	for senderType := range ns.notificationSenders {
		supportedTypes = append(supportedTypes, senderType)
	}

	return supportedTypes
}