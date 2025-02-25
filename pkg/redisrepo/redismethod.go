package redisrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"gochatapp/model"

	"github.com/go-redis/redis/v8"
)

func RegisterNewUser(username, password string) error {
	// redis-cli
	// SYNTAX: SET key value
	// SET username password
	// register new username:password key-value pair
	err := redisClient.Set(context.Background(), username, password, 0).Err()
	if err != nil {
		log.Println("error while adding new user", err)
		return err
	}

	// redis-cli
	// SYNTAX: SADD key value
	// SADD users username
	err = redisClient.SAdd(context.Background(), userSetKey(), username).Err()
	if err != nil {
		log.Println("error while adding user in set", err)
		// redis-cli
		// SYNTAX: DEL key
		// DEL username
		// drop the registered user
		redisClient.Del(context.Background(), username)

		return err
	}

	return nil
}

func IsUserExist(username string) bool {
	// redis-cli
	// SYNTAX: SISMEMBER key value
	// SISMEMBER users username
	return redisClient.SIsMember(context.Background(), userSetKey(), username).Val()
}

func IsUserAuthentic(username, password string) error {
	// redis-cli
	// SYNTAX: GET key
	// GET username
	p := redisClient.Get(context.Background(), username).Val()

	if !strings.EqualFold(p, password) {
		return fmt.Errorf("invalid username or password")
	}

	return nil
}

// UpdateContactList add contact to username's contact list
// if not present or update its timestamp as last contacted
func UpdateContactList(username, contact string) error {
	zs := &redis.Z{Score: float64(time.Now().Unix()), Member: contact}

	// redis-cli SCORE is always float or int
	// SYNTAX: ZADD key SCORE MEMBER
	// ZADD contacts:username 1661360942123 contact
	err := redisClient.ZAdd(context.Background(),
		contactListZKey(username),
		zs,
	).Err()

	if err != nil {
		log.Println("error while updating contact list. username: ",
			username, "contact:", contact, err)
		return err
	}

	return nil
}

func CreateChat(c *model.Chat) (string, error) {
	chatKey := chatKey()
	fmt.Println("chat key", chatKey)

	by, _ := json.Marshal(c)

	// Instead of using JSON.SET, use regular SET with JSON string
	res, err := redisClient.Set(
		context.Background(),
		chatKey,
		string(by),
		0,
	).Result()

	if err != nil {
		log.Println("error while setting chat json", err)
		return "", err
	}

	log.Println("chat successfully set", res)

	// Store additional keys for lookups
	// Format: chat:from:to:timestamp
	lookupKey := fmt.Sprintf("lookup:%s:%s:%d", c.From, c.To, c.Timestamp)
	err = redisClient.Set(context.Background(), lookupKey, chatKey, 0).Err()
	if err != nil {
		log.Println("error storing lookup key", err)
	}

	// Store reverse lookup too (for bidirectional chat)
	reverseLookupKey := fmt.Sprintf("lookup:%s:%s:%d", c.To, c.From, c.Timestamp)
	err = redisClient.Set(context.Background(), reverseLookupKey, chatKey, 0).Err()
	if err != nil {
		log.Println("error storing reverse lookup key", err)
	}

	// add contacts to both user's contact list
	err = UpdateContactList(c.From, c.To)
	if err != nil {
		log.Println("error while updating contact list of", c.From)
	}

	err = UpdateContactList(c.To, c.From)
	if err != nil {
		log.Println("error while updating contact list of", c.To)
	}

	return chatKey, nil
}

func CreateFetchChatBetweenIndex() {
	log.Println("RediSearch not available - skipping index creation")
	// No action needed since we're not using RediSearch
}

func FetchChatBetween(username1, username2, fromTS, toTS string) ([]model.Chat, error) {
	// We'll use pattern matching to find keys
	fromTSInt, err := parseTimestamp(fromTS)
	if err != nil {
		fromTSInt = 0 // Default to 0 if parsing fails
	}
	
	toTSInt, err := parseTimestamp(toTS)
	if err != nil {
		toTSInt = time.Now().Unix() // Default to current time if parsing fails
	}
	
	// Get all lookup keys for these users
	pattern1 := fmt.Sprintf("lookup:%s:%s:*", username1, username2)
	pattern2 := fmt.Sprintf("lookup:%s:%s:*", username2, username1)
	
	// Find all matching keys
	keys1, err := redisClient.Keys(context.Background(), pattern1).Result()
	if err != nil {
		return nil, err
	}
	
	keys2, err := redisClient.Keys(context.Background(), pattern2).Result()
	if err != nil {
		return nil, err
	}
	
	// Combine and filter by timestamp
	allKeys := append(keys1, keys2...)
	var validChatKeys []string
	
	for _, key := range allKeys {
		parts := strings.Split(key, ":")
		if len(parts) != 4 {
			continue
		}
		
		ts, err := parseTimestamp(parts[3])
		if err != nil {
			continue
		}
		
		if ts >= fromTSInt && ts <= toTSInt {
			// Get the chat key from the lookup
			chatKey, err := redisClient.Get(context.Background(), key).Result()
			if err != nil {
				continue
			}
			validChatKeys = append(validChatKeys, chatKey)
		}
	}
	
	// Get the actual chat data
	var chats []model.Chat
	for _, chatKey := range validChatKeys {
		chatData, err := redisClient.Get(context.Background(), chatKey).Result()
		if err != nil {
			continue
		}
		
		var chat model.Chat
		err = json.Unmarshal([]byte(chatData), &chat)
		if err != nil {
			continue
		}
		
		chats = append(chats, chat)
	}
	
	// Sort by timestamp (newest first)
	sortChatsByTimestampDesc(chats)
	
	return chats, nil
}

// FetchContactList of the user. It includes all the messages sent to and received by contact
// It will return a sorted list by last activity with a contact
func FetchContactList(username string) ([]model.ContactList, error) {
	zRangeArg := redis.ZRangeArgs{
		Key:   contactListZKey(username),
		Start: 0,
		Stop:  -1,
		Rev:   true,
	}

	// redis-cli
	// SYNTAX: ZRANGE key from_index to_index REV WITHSCORES
	// ZRANGE contacts:username 0 -1 REV WITHSCORES
	res, err := redisClient.ZRangeArgsWithScores(context.Background(), zRangeArg).Result()

	if err != nil {
		log.Println("error while fetching contact list. username: ",
			username, err)
		return nil, err
	}

	contactList := DeserialiseContactList(res)

	return contactList, nil
}

// Helper function to parse timestamp string to int64
func parseTimestamp(ts string) (int64, error) {
	if ts == "+inf" || ts == "inf" {
		return time.Now().Unix(), nil
	}
	if ts == "-inf" {
		return 0, nil
	}
	var tsInt int64
	_, err := fmt.Sscanf(ts, "%d", &tsInt)
	return tsInt, err
}

// Helper function to sort chats by timestamp in descending order
func sortChatsByTimestampDesc(chats []model.Chat) {
	for i := 0; i < len(chats)-1; i++ {
		for j := i + 1; j < len(chats); j++ {
			if chats[i].Timestamp < chats[j].Timestamp {
				chats[i], chats[j] = chats[j], chats[i]
			}
		}
	}
}