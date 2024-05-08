package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

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
		options.TokenExpiryInMinutes = 2 // 2 minutes default token expiry
	}
	options.TokenExpiryInMinutes = tokenExpiryInMinutes

	profileRepo := persist.NewProfileRepository(bootDB(options))
	mqclient := mq.NewOtpMQClient(bootMQ(options), profileRepo)
	authenticator := internal.NewAuthenticator(options.TokenExpiryInMinutes)
	authSvc := internal.NewAuthService(profileRepo, mqclient, authenticator)
	interceptors := connect.WithInterceptors(internal.NewTokenInterceptor(authenticator))

	go mqclient.Consume()

	mux := http.NewServeMux()
	mux.Handle(authv1connect.NewAuthServiceHandler(authSvc, interceptors))

	mux2 := http.NewServeMux()
	mux2.Handle(API_Prefix, http.StripPrefix("/api", mux))

	log.Printf("listening at localhost:%s\n", options.Port)
	if err := http.ListenAndServe(fmt.Sprintf("localhost:%s", options.Port), mux2); err != nil {
		log.Panicf("failed to launch auth-service: %v", err)
	}
}

func bootDB(options *Options) *gorm.DB {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		options.DBHost, options.DBUsername, options.DBPassword, options.DBName, options.DBPort)

	db, err := gorm.Open(postgres.Open(dsn))
	if err != nil {
		log.Fatalf("failed to open db connection, %v", err)
	}
	db.AutoMigrate(&models.Profile{})
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
