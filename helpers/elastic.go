package helpers

import (
	"bytes"
	"encoding/json"
	"log"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
)

type User struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
}

type Event struct {
	Type     string `json:"type"`
	Category string `json:"category"`
	Target   string `json:"target"`
	Status   string `json:"status"`
}

type AuditLog struct {
	Timestamp time.Time `json:"@timestamp"`
	User      User      `json:"user"`
	Event     Event     `json:"event"`
}

type ElasticSearch struct {
	Enabled bool
	Client  *elasticsearch.Client
	Index   string
}

func CreateElasticSearchLog(es ElasticSearch, timestamp time.Time, user string, ip string, eventType string, category string, target string, status string) {
	if !es.Enabled {
		return
	}
	audit := AuditLogObject(timestamp, user, ip, eventType, category, target, status)

	data, err := json.Marshal(audit)
	if err != nil {
		log.Println("[ERROR] Error while sending audit log to ElasticSearch")
		log.Println(err)
	}

	res, err := es.Client.Index(es.Index, bytes.NewReader(data))
	if err != nil {
		log.Println("[ERROR] Error while sending audit log to ElasticSearch")
		log.Println(err)
	}

	defer res.Body.Close()
}

func AuditLogObject(timestamp time.Time, user string, ip string, eventType string, category string, target string, status string) *AuditLog {
	return &AuditLog{
		Timestamp: timestamp,
		User: User{
			Name: user,
			IP:   ip,
		},
		Event: Event{
			Type:     eventType,
			Category: category,
			Target:   target,
			Status:   status,
		},
	}
}
