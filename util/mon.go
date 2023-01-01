package util

import (
	"context"
	"fmt"
	"os"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func MongoClient() *mongo.Client {
	host := os.Getenv("MONGO_HOST")
	user := os.Getenv("MONGO_USER")
	password := os.Getenv("MONGO_PWD")
	database := os.Getenv("MONGO_DB")
	// rs := os.Getenv("MONGO_RS")

	options := &options.ClientOptions{
		Hosts: []string{
			host,
		},
		Auth: &options.Credential{
			Username:   user,
			Password:   password,
			AuthSource: database,
		},
		MaxPoolSize: &[]uint64{10}[0],
	}

	client, err := mongo.Connect(context.TODO(), options)

	if err != nil {
		panic(err)
	}

	// defer func() {
	// 	if err := client.Disconnect(context.TODO()); err != nil {
	// 		panic(err)
	// 	}
	// }()

	return client
}

func GetDocs(client *mongo.Collection, filter bson.E) *mongo.Cursor {
	docs, err := client.Find(context.TODO(), bson.D{filter}, options.Find().SetLimit(10))

	if err != nil {
		fmt.Printf("%+v", err)
		return nil
	}

	return docs
}
