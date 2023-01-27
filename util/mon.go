package util

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Leaders struct {
	Date        time.Time
	Solved      []GamesStats `bson:"solved,omitempty"`
	Created     []GamesStats `bson:"created,omitempty"`
	UsersSolved []GamesStats `bson:"usersSolved,omitempty"`
}

type GamesStats struct {
	User   string `bson:"user"`
	Amount int    `bson:"amount"`
}

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

func AggregateLeaders(date time.Time, sortKey string, limit int) []bson.M {
	leadersColl := MongoClient().Database("slack").Collection("leaders")
	firstDay := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, time.Local)
	lastDay := firstDay.AddDate(0, 1, 0).Add(time.Nanosecond * -1)

	aggFilter := []bson.M{{
		"$sort": bson.M{
			sortKey: -1,
		},
	}, {
		"$match": bson.M{
			"date": bson.M{
				"$gte": firstDay,
				"$lte": lastDay,
			},
		},
	}}

	if limit != -1 {
		aggFilter = append(aggFilter, bson.M{"$limit": limit})
	}

	var solved []bson.M

	solvedAgg, err := leadersColl.Aggregate(context.TODO(), aggFilter)

	if err != nil {

	}

	solvedAgg.All(context.TODO(), &solved)
	return solved
}
