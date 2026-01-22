package uvmongo

import (
	"context"
	"log/slog"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	uvkafka "univie.ac.at/efrsw/consumer/uv_kafka"
)

const (
	databaseName  string = "imagedbplantconsumer"
	potatoColName string = "imagecolplantpotato"
	pepperColName string = "imagecolplantpepper"
	tomatoColName string = "imagecolplanttomato"
)

type MongoConnection struct {
	c *mongo.Client
}

// Initialize connection to a MongoDB instance
func (mc *MongoConnection) Initialize(ctx context.Context, mongoUri string) error {
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(mongoUri).SetServerAPIOptions(serverAPI)
	client, err := mongo.Connect(opts)

	if err != nil {
		return err
	}

	mc.c = client
	if err := mc.Ping(ctx); err != nil {
		mc.c = nil
		return err
	}
	slog.Info("Successfully connected to MongoDB")

	return nil
}

// Close connection to the instance
func (mc *MongoConnection) Close(ctx context.Context) error {
	return mc.c.Disconnect(ctx)
}

// Sends a ping to confirm a successful connection
func (mc *MongoConnection) Ping(ctx context.Context) error {
	var result bson.M
	if err := mc.c.Database(databaseName).RunCommand(
		ctx,
		bson.D{{"ping", 1}},
	).Decode(&result); err != nil {
		return err
	}

	return nil
}

// Listen to messages in the channel and update info in the consumerdb
func (mc *MongoConnection) HandleMessage(ctx context.Context, mb chan uvkafka.LeafUpdateEvent) {
	for {
		select {
		case <-ctx.Done():
			slog.Info("Shutting down message handler")
			return
		case e, ok := <-mb:
			if !ok {
				slog.Info("Message bus closed")
				return
			}

			doc := &LeafDocument{}
			doc.FromMessage(e.Message)

			collectionName := mc.getCollectionName(e.Topic)
			if collectionName == "" {
				slog.Warn("Skipping unknown topic", "topic", e.Topic)
				continue
			}

			// Update mongo doc
			opts := options.Replace().SetUpsert(true)
			_, err := mc.c.Database(databaseName).Collection(collectionName).ReplaceOne(
				ctx,
				bson.D{{"id", e.Message.Id}},
				doc,
				opts,
			)

			if err != nil {
				slog.Error("Failed to upsert message",
					"messageId", e.Message.Id,
					"topic", e.Topic,
					"cameraId", e.Message.CameraId,
					"error", err)
				continue // IF there's error -> don't commit offset
			}

			slog.Info("Upserted document",
				"id", e.Message.Id,
				"topic", e.Topic,
				"disease", e.Message.Disease)

			e.OnSuccess() // Commit offset upon successful upsert
		}
	}
}

// Retrieves plant by Id
func (mc *MongoConnection) GetPlantById(ctx context.Context, plantType string, imageId int) (*LeafDocument, error) {
	collectionName := mc.getCollectionName(plantType)
	if collectionName == "" {
		return nil, mongo.ErrNoDocuments
	}

	var doc LeafDocument
	err := mc.c.Database(databaseName).Collection(collectionName).FindOne(
		ctx,
		bson.D{{"id", imageId}},
	).Decode(&doc)

	if err != nil {
		return nil, err
	}

	return &doc, nil
}

// Get amount of docs in a collection
func (mc *MongoConnection) GetTotalPlants(ctx context.Context, plantType string) (int64, error) {
	collectionName := mc.getCollectionName(plantType)
	if collectionName == "" {
		return 0, mongo.ErrNoDocuments
	}

	count, err := mc.c.Database(databaseName).Collection(collectionName).CountDocuments(
		ctx,
		bson.D{},
	)

	if err != nil {
		return 0, err
	}

	return count, nil
}

// Determine collection name based on topic
func (mc *MongoConnection) getCollectionName(plantType string) string {
	switch plantType {
	case "potato":
		return potatoColName
	case "pepper":
		return pepperColName
	case "tomato":
		return tomatoColName
	default:
		return ""
	}
}
