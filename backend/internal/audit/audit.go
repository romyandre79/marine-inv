package audit

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
)

type AuditLogPayload struct {
	UserID     *string `json:"user_id"`
	Action     string  `json:"action"`
	EntityType string  `json:"entity_type"`
	EntityID   *string `json:"entity_id"`
	Details    string  `json:"details"`
}

func SendAuditLog(authHeader string, action string, entityType string, entityID *string, details string) {
	mmsURL := os.Getenv("MMS_API_URL")
	if mmsURL == "" {
		mmsURL = "http://localhost:3004" // fallback to MMS Backend
	}

	payload := AuditLogPayload{
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Details:    details,
	}

	log.Printf("[AUDIT] Preparing to send audit log: %s, URL: %s/api/v1/logs", action, mmsURL)

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[AUDIT] JSON Marshal error: %v", err)
		return
	}

	req, err := http.NewRequest("POST", mmsURL+"/api/v1/logs", bytes.NewBuffer(jsonBytes))
	if err != nil {
		log.Printf("[AUDIT] HTTP Request creation error: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[AUDIT] HTTP Post error: %v", err)
		return
	}
	defer resp.Body.Close()

	log.Printf("[AUDIT] Response Status: %s", resp.Status)
}
