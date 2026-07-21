// MongoDB adapter demo. Requires a reachable MongoDB at MONGO_URI
// (defaults to mongodb://localhost:27017). Run:
//
//   go run ./examples/mongo
//
// Each command issued by the driver becomes a boeng operation named
// "mongo.<command>". No OpenTelemetry import in the application code.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"
	boengmongo "github.com/Maximumsoft-Co-LTD/obs-brutal/boeng/mongo"
)

func main() {
	defer boeng.Init(boeng.Config{Service: "mongo_demo", Env: "dev"}).Close()

	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().
		ApplyURI(uri).
		SetMonitor(boengmongo.CommandMonitor()))
	if err != nil {
		fmt.Println("connect:", err)
		return
	}
	defer client.Disconnect(context.Background())

	coll := client.Database("boeng_demo").Collection("users")
	_, _ = coll.InsertOne(ctx, bson.M{"name": "demo", "ts": time.Now()})
	_ = coll.FindOne(ctx, bson.M{"name": "demo"}).Err()
}
