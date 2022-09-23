package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (ns *notificationServer) getSubscribersHandler(w http.ResponseWriter, r *http.Request) {
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

	numAllKeys, _ := ns.redisClient.DBSize(r.Context()).Result()
	allKeys, _, _ := ns.redisClient.Scan(r.Context(), 0, "*", numAllKeys).Result()

	var subscribers []string
	for _, key := range allKeys {
		if (!strings.HasPrefix(key, "msg-")) && (key != "counters") {
			subscribers = append(subscribers, key)
		}
	}

	jsonSubs, err := json.Marshal(subscribers)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}

	w.Write(jsonSubs)
}
