package services

import "encoding/json"

type NotifyService struct {
	NotifyType string `json:"notify_type"`
	Message    string `json:"message"`
}

// SendWsNotify - отправка уведомления через WebSocket
func SendWsNotify(userID int64, notifyType string, message string) error {
	if len(notifyType) == 0 {
		notifyType = "info"
	}
	if len(message) == 0 {
		return nil
	}
	if len(message) > 100 {
		message = message[:100] + "..."
	}
	// Формируем сообщение в формате JSON
	notify := NotifyService{NotifyType: notifyType, Message: message}
	jsonData, err := json.Marshal(notify)
	if err != nil {
		return err
	}
	GlobalWSConnManager.Send(userID, jsonData)
	return nil
}
