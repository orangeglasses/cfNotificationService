package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"
)

type Stats struct {
	MessagesStored  int64            `json:"messages_stored"`
	UsersSubscribed int64            `json:"users_subscribed"`
	MsgSent         map[string]int64 `json:"messages_sent"`
}

type StatsCollector struct {
	redisClient *redis.Client

	MessagesStoredDesc  *prometheus.Desc
	UsersSubscribedDesc *prometheus.Desc
	MsgSentDesc         map[string]*prometheus.Desc
}

func NewStatsCollector(rc *redis.Client) *StatsCollector {
	labels := prometheus.Labels{"app": "cfNotificationService"}
	s := &StatsCollector{
		MessagesStoredDesc: prometheus.NewDesc(prometheus.BuildFQName("cfNotificationService", "stats", "messages_stored"),
			"Number of messages currently stored",
			nil,
			labels,
		),
		UsersSubscribedDesc: prometheus.NewDesc(prometheus.BuildFQName("cfNotificationService", "stats", "users_subscribed"),
			"Number of messages currently stored",
			nil,
			labels,
		),
		MsgSentDesc: make(map[string]*prometheus.Desc),
	}

	stats := s.Get(context.Background())

	for counterKey := range stats.MsgSent {
		s.MsgSentDesc[counterKey] = prometheus.NewDesc(prometheus.BuildFQName("cfNotificationService", "stats", "messages_sent_"+counterKey),
			"Number of "+counterKey+"messages sent",
			nil,
			labels,
		)
	}

	return s
}

func (s *StatsCollector) Get(ctx context.Context) Stats {
	var stats Stats

	counterKeys, _ := s.redisClient.HKeys(ctx, "counters").Result()
	numAllKeys, _ := s.redisClient.DBSize(ctx).Result()
	msgKeys, _, _ := s.redisClient.Scan(ctx, 0, "msg-*", numAllKeys).Result()
	numMsgKeys := int64(len(msgKeys))
	numUsers := numAllKeys - numMsgKeys - 1

	stats = Stats{
		MessagesStored:  numMsgKeys,
		UsersSubscribed: numUsers,
		MsgSent:         make(map[string]int64),
	}

	for _, counterKey := range counterKeys {
		counterValString, err := s.redisClient.HGet(ctx, "counters", counterKey).Result()
		if err != nil {
			counterValString = "0"
		}

		counterVal, err := strconv.Atoi(counterValString)
		if err != nil {
			log.Printf("error converting string to number: %v\n", err.Error())
		}

		stats.MsgSent[counterKey] = int64(counterVal)
	}

	return stats
}

func (s *StatsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- s.MessagesStoredDesc
	ch <- s.UsersSubscribedDesc
	for _, counterDesc := range s.MsgSentDesc {
		ch <- counterDesc
	}
}

func (s *StatsCollector) Collect(ch chan<- prometheus.Metric) {
	stats := s.Get(context.Background())

	ch <- prometheus.MustNewConstMetric(
		s.MessagesStoredDesc,
		prometheus.CounterValue,
		float64(stats.MessagesStored),
	)
	ch <- prometheus.MustNewConstMetric(
		s.UsersSubscribedDesc,
		prometheus.CounterValue,
		float64(stats.UsersSubscribed),
	)

	for counterName, counterValue := range stats.MsgSent {
		ch <- prometheus.MustNewConstMetric(
			s.MsgSentDesc[counterName],
			prometheus.CounterValue,
			float64(counterValue),
		)
	}
}

func (s *StatsCollector) statsHandler(w http.ResponseWriter, r *http.Request) {
	stats := s.Get(r.Context())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
