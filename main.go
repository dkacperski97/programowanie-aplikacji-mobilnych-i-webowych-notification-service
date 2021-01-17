package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"

	"example.com/notification-service/handlers"
	"example.com/notification-service/models"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

var client *redis.Client

var upgrader = websocket.Upgrader{
	CheckOrigin: func(req *http.Request) bool {
		origin := req.Header.Get("Origin")
		return origin == os.Getenv("APP_1_ORIGIN") || origin == os.Getenv("APP_2_ORIGIN")
	},
}

func getNotifications(w http.ResponseWriter, req *http.Request) {
	claims, _ := handlers.GetClaims(req.Context())
	if claims.Role != "sender" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	c, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	defer func() {
		c.Close()
	}()
	models.WatchNotifications(client, c, claims.User)
}

func addNotification(w http.ResponseWriter, req *http.Request) {
	claims, _ := handlers.GetClaims(req.Context())

	if claims.Role != "courier" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	headerContentType := req.Header.Get("Content-Type")
	if headerContentType != "application/json" {
		http.Error(w, "Content Type is not application/json", http.StatusUnsupportedMediaType)
		return
	}

	var notification models.Notification

	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&notification)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	validationErr, err := models.IsNotificationValid(notification.User, notification.Message)
	if validationErr != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	err = notification.Save(client)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Location", "/notifications")
}

func getRedisClient() *redis.Client {
	redisPort := os.Getenv("REDIS_PORT")
	if redisPort == "" {
		redisPort = "6379"
	}
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost"
	}
	redisPass := os.Getenv("REDIS_PASS")
	redisDbString := os.Getenv("REDIS_DB")
	redisDb, err := strconv.Atoi(redisDbString)
	if err != nil {
		redisDb = 0
	}
	return redis.NewClient(&redis.Options{
		Addr:     redisHost + ":" + redisPort,
		Password: redisPass,
		DB:       redisDb,
	})
}

func main() {
	err := godotenv.Load()
	if err == nil {
		log.Print(".env file loaded")
	}

	client = getRedisClient()
	defer client.Close()

	mainRouter := mux.NewRouter()

	mainRouter.HandleFunc("/notifications", http.HandlerFunc(addNotification)).Methods(http.MethodPost, http.MethodOptions)
	mainRouter.HandleFunc("/notifications/ws", http.HandlerFunc(getNotifications))
	mainRouter.Use(mux.CORSMethodMiddleware(mainRouter))
	mainRouter.Use(handlers.JwtHandler([]byte(os.Getenv("JWT_SECRET")), true))
	mainRouter.Use(handlers.HeadersHandler())

	http.Handle("/", mainRouter)

	port := os.Getenv("PORT")
	if port == "" {
		port = "5500"
	}

	s := &http.Server{
		Addr:    ":" + port,
		Handler: nil,
	}

	log.Println("Listening on :" + port + " ...")
	err = s.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
}
