package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"connectrpc.com/connect"
	"github.com/ilivestrong/auth-service/internal"

	"github.com/ilivestrong/auth-service/internal/models"
	"github.com/ilivestrong/auth-service/internal/persist"
	"github.com/ilivestrong/auth-service/internal/protos/gen/auth/v1/authv1connect"
	mq "github.com/ilivestrong/auth-service/internal/rabbitmq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/joho/godotenv"
	amqp "github.com/rabbitmq/amqp091-go"
)

type (
	Options struct {
		AMQPAddress          string
		DBHost               string
		DBName               string
		DBUsername           string
		DBPassword           string
		DBPort               string
		Port                 string
		TokenExpiryInMinutes int
	}
)

const API_Prefix = "/api/"

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	options := &Options{
		AMQPAddress: mustGetEnv("AMQP_ADDRESS"),
		DBHost:      mustGetEnv("DB_HOST"),
		DBName:      mustGetEnv("DB_NAME"),
		DBUsername:  mustGetEnv("DB_USERNAME"),
		DBPassword:  mustGetEnv("DB_PASSWORD"),
		DBPort:      mustGetEnv("DB_PORT"),
		Port:        mustGetEnv("PORT"),
	}

	tokenExpiryInMinutes, err := strconv.Atoi(mustGetEnv("TOKEN_EXPIRY_IN_MINUTES"))
	if err != nil {
		options.TokenExpiryInMinutes = 2
	}
	options.TokenExpiryInMinutes = tokenExpiryInMinutes

	loggedInUsersCache := internal.NewInMemoryCache()

	db := bootDB(options)
	profileRepo := persist.NewProfileRepository(db)
	eventRepo := persist.NewEventRepository(db)

	amqp := bootMQ(options)
	mqclient := mq.NewOtpMQClient(amqp, profileRepo)
	authenticator := internal.NewAuthenticator(options.TokenExpiryInMinutes)
	authSvc := internal.NewAuthService(
		profileRepo,
		eventRepo,
		mqclient,
		authenticator,
		loggedInUsersCache,
	)
	interceptors := connect.WithInterceptors(internal.NewTokenInterceptor(authenticator, loggedInUsersCache))

	go mqclient.Consume()

	mux := http.NewServeMux()
	mux.Handle(authv1connect.NewAuthServiceHandler(authSvc, interceptors))

	mux2 := http.NewServeMux()
	mux2.Handle(API_Prefix, http.StripPrefix("/api", mux))

	log.Printf("listening at localhost:%s\n", options.Port)
	go http.ListenAndServe(fmt.Sprintf("localhost:%s", options.Port), mux2)

	shutdownOnSignal(db, amqp)
}

func bootDB(options *Options) *gorm.DB {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		options.DBHost, options.DBUsername, options.DBPassword, options.DBName, options.DBPort)

	db, err := gorm.Open(postgres.Open(dsn))
	if err != nil {
		log.Fatalf("failed to open db connection, %v", err)
	}
	db.AutoMigrate(&models.Profile{}, models.Event{})
	return db
}

func bootMQ(options *Options) *amqp.Connection {
	conn, err := amqp.Dial(options.AMQPAddress)
	if err != nil {
		log.Fatalf("failed to connect to RabbitMQ, %v", err)
	}
	return conn
}

func mustGetEnv(key string) string {
	v, ok := os.LookupEnv(key)
	if !ok {
		log.Fatalf("failed to get env for: %s", key)
	}
	return v
}

func waitForShutdownSignal() string {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	sig := <-c

	return sig.String()
}

func shutdownOnSignal(db *gorm.DB, amqp *amqp.Connection) {
	signalName := waitForShutdownSignal()
	fmt.Printf("recieved signal: %s starting shutdown...\n", signalName)

	if db != nil {
		if dbIns, err := db.DB(); err == nil {
			dbIns.Close()
			log.Println("db connection closed")
		}
	}

	if amqp != nil {
		if err := amqp.Close(); err == nil {
			log.Println("amqp connection closed")
		}
	}
}
