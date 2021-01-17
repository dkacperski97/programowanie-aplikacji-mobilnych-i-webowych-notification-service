package models

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 30 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

type Notification struct {
	User    string `json:"user"`
	Message string `json:"message"`
}

func CreateNotification(user, message string) (*Notification, error, error) {
	validationErr, err := IsNotificationValid(user, message)
	if validationErr != nil {
		return nil, validationErr, nil
	}
	if err != nil {
		return nil, nil, err
	}
	n := new(Notification)
	n.User = user
	n.Message = message
	return n, nil, nil
}

func IsNotificationValid(user, message string) (error, error) {
	if user == "" {
		return errors.New("Incorrect user"), nil
	}
	if message == "" {
		return errors.New("Incorrect message"), nil
	}
	return nil, nil
}

func (notification *Notification) Save(client *redis.Client) error {
	return client.Publish(context.Background(), "notifications:"+notification.User, notification.Message).Err()
}

func WatchNotifications(client *redis.Client, ws *websocket.Conn, user string) {
	ctx := context.Background()
	pubsub := client.PSubscribe(ctx, "notifications:"+user)
	_, err := pubsub.Receive(ctx)
	if err != nil {
		panic(err)
	}
	ch := pubsub.Channel()
	ticker := time.NewTicker(pingPeriod)

	go readPong(ws)

	defer func() {
		ticker.Stop()
		_ = pubsub.Close()
		ws.Close()
	}()
	for {
		select {
		case msg := <-ch:
			ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := ws.WriteMessage(websocket.TextMessage, []byte(msg.Payload)); err != nil {
				return
			}
		case <-ticker.C:
			ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func readPong(ws *websocket.Conn) {
	defer func() {
		ws.Close()
	}()
	ws.SetReadDeadline(time.Now().Add(pongWait))
	ws.SetPongHandler(func(string) error { ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
	}
}
