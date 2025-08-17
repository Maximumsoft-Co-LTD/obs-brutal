package example

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"obs-brutal/obsvbrutal"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// RepositoryExample demonstrates database logging
type RepositoryExample struct {
	logger      obsvbrutal.Logger
	provider    *obsvbrutal.OTelProvider
	mongoClient *mongo.Client
	redisClient *redis.Client
}

// NewRepositoryExample creates repository example
func NewRepositoryExample(
	logger obsvbrutal.Logger,
	provider *obsvbrutal.OTelProvider,
	redisClient *redis.Client,
) *RepositoryExample {
	return &RepositoryExample{
		logger:      logger.Mod("repository"),
		provider:    provider,
		redisClient: redisClient,
	}
}

// InitMongoDB initializes MongoDB connection
func (r *RepositoryExample) InitMongoDB(ctx context.Context, uri string) error {
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}

	r.logger.
		F("uri", uri).
		Info("Connecting to MongoDB")

	clientOptions := options.Client().
		ApplyURI(uri).
		SetConnectTimeout(10 * time.Second).
		SetServerSelectionTimeout(5 * time.Second)

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		r.logger.Err(err).Error("Failed to connect to MongoDB")
		return err
	}

	// Ping to verify connection
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx, nil); err != nil {
		r.logger.Err(err).Error("Failed to ping MongoDB")
		return err
	}

	r.mongoClient = client
	r.logger.Info("Connected to MongoDB successfully")

	return nil
}

// User represents a user document
type User struct {
	ID        string    `bson:"_id,omitempty" json:"id"`
	Email     string    `bson:"email" json:"email"`
	Name      string    `bson:"name" json:"name"`
	Role      string    `bson:"role" json:"role"`
	Active    bool      `bson:"active" json:"active"`
	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
}

// CreateUser creates a user in MongoDB
func (r *RepositoryExample) CreateUser(ctx context.Context, user *User) error {
	// Start span
	ctx, span, logger := obsvbrutal.StartSpanWithLogger(ctx, r.logger, "mongodb.create_user")
	defer span.End()

	logger.
		F("user_email", user.Email).
		F("user_name", user.Name).
		Debug("Creating user in MongoDB")

	// Set timestamps
	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now

	// Insert user
	collection := r.mongoClient.Database("example").Collection("users")
	result, err := collection.InsertOne(ctx, user)
	if err != nil {
		logger.Err(err).Error("Failed to create user in MongoDB")
		obsvbrutal.RecordError(span, err, "MongoDB insert failed")
		return err
	}

	user.ID = result.InsertedID.(string)

	logger.
		F("user_id", user.ID).
		Info("User created in MongoDB")

	// Cache in Redis
	r.cacheUser(ctx, user)

	return nil
}

// GetUser gets a user by ID
func (r *RepositoryExample) GetUser(ctx context.Context, userID string) (*User, error) {
	// Start span
	ctx, span, logger := obsvbrutal.StartSpanWithLogger(ctx, r.logger, "repository.get_user")
	defer span.End()

	logger.
		F("user_id", userID).
		Debug("Getting user")

	// Try Redis cache first
	if user := r.getCachedUser(ctx, userID); user != nil {
		logger.
			F("user_id", userID).
			Debug("User found in cache")
		return user, nil
	}

	// Get from MongoDB
	_, mongoSpan := obsvbrutal.StartSpan(ctx, "mongodb.find_user")
	defer mongoSpan.End()

	collection := r.mongoClient.Database("example").Collection("users")

	var user User
	err := collection.FindOne(ctx, bson.M{"_id": userID}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			logger.
				F("user_id", userID).
				Warn("User not found in MongoDB")
			return nil, nil
		}
		logger.
			Err(err).
			F("user_id", userID).
			Error("Failed to get user from MongoDB")
		obsvbrutal.RecordError(mongoSpan, err, "MongoDB find failed")
		return nil, err
	}

	logger.
		F("user_id", userID).
		Info("User retrieved from MongoDB")

	// Cache the user
	r.cacheUser(ctx, &user)

	return &user, nil
}

// UpdateUser updates a user
func (r *RepositoryExample) UpdateUser(ctx context.Context, userID string, updates map[string]interface{}) error {
	// Start span
	ctx, span, logger := obsvbrutal.StartSpanWithLogger(ctx, r.logger, "mongodb.update_user")
	defer span.End()

	logger.
		F("user_id", userID).
		F("updates", updates).
		Debug("Updating user in MongoDB")

	// Add updated timestamp
	updates["updated_at"] = time.Now()

	// Update in MongoDB
	collection := r.mongoClient.Database("example").Collection("users")
	result, err := collection.UpdateOne(
		ctx,
		bson.M{"_id": userID},
		bson.M{"$set": updates},
	)

	if err != nil {
		logger.
			Err(err).
			F("user_id", userID).
			Error("Failed to update user in MongoDB")
		obsvbrutal.RecordError(span, err, "MongoDB update failed")
		return err
	}

	if result.MatchedCount == 0 {
		logger.
			F("user_id", userID).
			Warn("User not found for update")
		return mongo.ErrNoDocuments
	}

	logger.
		F("user_id", userID).
		F("modified_count", result.ModifiedCount).
		Info("User updated in MongoDB")

	// Invalidate cache
	r.invalidateUserCache(ctx, userID)

	return nil
}

// Redis operations

func (r *RepositoryExample) cacheUser(ctx context.Context, user *User) {
	if r.redisClient == nil {
		return
	}

	logger := r.getLogger(ctx)

	// Start span
	ctx, span := obsvbrutal.StartSpan(ctx, "redis.cache_user")
	defer span.End()

	key := fmt.Sprintf("user:%s", user.ID)
	data, err := json.Marshal(user)
	if err != nil {
		logger.Err(err).Error("Failed to marshal user for caching")
		return
	}

	err = r.redisClient.Set(ctx, key, data, 5*time.Minute).Err()
	if err != nil {
		logger.
			Err(err).
			F("key", key).
			Warn("Failed to cache user in Redis")
		return
	}

	logger.
		F("key", key).
		F("ttl", "5m").
		Debug("User cached in Redis")
}

func (r *RepositoryExample) getCachedUser(ctx context.Context, userID string) *User {
	if r.redisClient == nil {
		return nil
	}

	logger := r.getLogger(ctx)

	// Start span
	ctx, span := obsvbrutal.StartSpan(ctx, "redis.get_cached_user")
	defer span.End()

	key := fmt.Sprintf("user:%s", userID)

	data, err := r.redisClient.Get(ctx, key).Result()
	if err != nil {
		if err != redis.Nil {
			logger.
				Err(err).
				F("key", key).
				Warn("Failed to get cached user from Redis")
		}
		return nil
	}

	var user User
	if err := json.Unmarshal([]byte(data), &user); err != nil {
		logger.
			Err(err).
			F("key", key).
			Error("Failed to unmarshal cached user")
		return nil
	}

	logger.
		F("key", key).
		Debug("User retrieved from Redis cache")

	return &user
}

func (r *RepositoryExample) invalidateUserCache(ctx context.Context, userID string) {
	if r.redisClient == nil {
		return
	}

	logger := r.getLogger(ctx)

	key := fmt.Sprintf("user:%s", userID)

	err := r.redisClient.Del(ctx, key).Err()
	if err != nil {
		logger.
			Err(err).
			F("key", key).
			Warn("Failed to invalidate user cache")
		return
	}

	logger.
		F("key", key).
		Debug("User cache invalidated")
}

// Session management example

func (r *RepositoryExample) CreateSession(ctx context.Context, userID, sessionID string, ttl time.Duration) error {
	if r.redisClient == nil {
		return fmt.Errorf("redis client not available")
	}

	logger := r.getLogger(ctx).
		F("user_id", userID).
		F("session_id", sessionID)

	logger.Debug("Creating session in Redis")

	// Create session data
	sessionData := map[string]interface{}{
		"user_id":    userID,
		"created_at": time.Now().Unix(),
		"last_seen":  time.Now().Unix(),
	}

	// Store session
	key := fmt.Sprintf("session:%s", sessionID)

	pipe := r.redisClient.Pipeline()
	pipe.HMSet(ctx, key, sessionData)
	pipe.Expire(ctx, key, ttl)

	// Add to user's sessions set
	userSessionsKey := fmt.Sprintf("user:sessions:%s", userID)
	pipe.SAdd(ctx, userSessionsKey, sessionID)
	pipe.Expire(ctx, userSessionsKey, ttl)

	_, err := pipe.Exec(ctx)
	if err != nil {
		logger.Err(err).Error("Failed to create session")
		return err
	}

	logger.
		F("ttl", ttl).
		Info("Session created successfully")

	return nil
}

// Rate limiting example

func (r *RepositoryExample) CheckRateLimit(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	if r.redisClient == nil {
		return true, nil // Allow if Redis not available
	}

	logger := r.getLogger(ctx).
		F("key", key).
		F("limit", limit).
		F("window", window)

	// Use sliding window counter
	now := time.Now().Unix()
	windowStart := now - int64(window.Seconds())

	pipe := r.redisClient.Pipeline()

	// Remove old entries
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart))

	// Count current window
	pipe.ZCard(ctx, key)

	// Add current request
	pipe.ZAdd(ctx, key, redis.Z{Score: float64(now), Member: now})

	// Set expiry
	pipe.Expire(ctx, key, window)

	cmds, err := pipe.Exec(ctx)
	if err != nil {
		logger.Err(err).Error("Failed to check rate limit")
		return true, err
	}

	// Get count from second command
	count := cmds[1].(*redis.IntCmd).Val()

	allowed := count < int64(limit)

	if !allowed {
		logger.
			F("count", count).
			Warn("Rate limit exceeded")
	} else {
		logger.
			F("count", count).
			Debug("Rate limit check passed")
	}

	return allowed, nil
}

// Helper method to get logger from context
func (r *RepositoryExample) getLogger(ctx context.Context) obsvbrutal.Logger {
	if logger, ok := obsvbrutal.GetLoggerFromContext(ctx); ok {
		return logger
	}
	return r.logger
}
