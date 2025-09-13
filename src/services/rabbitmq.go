package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	rabbitConn    *amqp.Connection
	rabbitChannel *amqp.Channel
	feedExchange  = "feed_events"
)

// FeedEvent - структура события для push feed
// (userID - кому отправить, postID, authorID, content, createdAt)
type FeedEvent struct {
	UserID    int64     `json:"user_id"`
	PostID    int64     `json:"post_id"`
	AuthorID  int64     `json:"author_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// InitRabbitMQ инициализирует соединение, exchange и очередь
func InitRabbitMQ() error {
	url := os.Getenv("RABBITMQ_URL")
	if url == "" {
		// Для тестового окружения используем порт 5673
		url = "amqp://guest:guest@localhost:5673/"
	}
	var err error
	rabbitConn, err = amqp.Dial(url)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}
	rabbitChannel, err = rabbitConn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open channel: %w", err)
	}
	// Создаем exchange типа topic
	if err := rabbitChannel.ExchangeDeclare(
		feedExchange,
		"topic",
		true,  // durable
		false, // auto-delete
		false, // internal
		false, // no-wait
		nil,   // args
	); err != nil {
		return fmt.Errorf("failed to declare exchange: %w", err)
	}
	log.Printf("RabbitMQ initialized successfully with URL: %s", url)
	return nil
}

// PublishFeedEvent публикует событие о новом посте для конкретного пользователя
func PublishFeedEvent(ctx context.Context, event FeedEvent) error {
	if rabbitChannel == nil {
		return fmt.Errorf("RabbitMQ channel not initialized")
	}
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	routingKey := fmt.Sprintf("user.%d", event.UserID)
	return rabbitChannel.PublishWithContext(ctx,
		feedExchange,
		routingKey,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
}

// StartFeedEventConsumer запускает воркер, который слушает события и пушит их через WebSocket
func StartFeedEventConsumer(ctx context.Context, queueName string) error {
	if rabbitChannel == nil {
		return fmt.Errorf("RabbitMQ channel not initialized")
	}
	// Создаем очередь с уникальным именем (или переданным)
	q, err := rabbitChannel.QueueDeclare(
		queueName,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}
	// Биндим очередь к exchange по routing key user.*
	if err := rabbitChannel.QueueBind(
		q.Name,
		"user.*",
		feedExchange,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("failed to bind queue: %w", err)
	}
	msgs, err := rabbitChannel.Consume(
		q.Name,
		"",
		true,  // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to start consumer: %w", err)
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-msgs:
				var event FeedEvent
				if err := json.Unmarshal(msg.Body, &event); err != nil {
					log.Println("Failed to unmarshal feed event:", err)
					continue
				}
				// Пушим событие через WebSocket
				// Формируем красивое событие для клиента
				pushMsg := struct {
					Event     string    `json:"event"`
					UserID    int64     `json:"user_id"`
					PostID    int64     `json:"post_id"`
					AuthorID  int64     `json:"author_id"`
					Content   string    `json:"content"`
					CreatedAt time.Time `json:"created_at"`
				}{
					Event:     "feed_posted",
					UserID:    event.UserID,
					PostID:    event.PostID,
					AuthorID:  event.AuthorID,
					Content:   event.Content,
					CreatedAt: event.CreatedAt,
				}
				pushData, _ := json.Marshal(pushMsg)
				GlobalWSConnManager.Send(event.UserID, pushData)
			}
		}
	}()
	return nil
}
