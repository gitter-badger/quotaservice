// Licensed under the Apache License, Version 2.0
// Details: https://raw.githubusercontent.com/maniksurtani/quotaservice/master/LICENSE

package redis

import (
	"testing"
	"github.com/maniksurtani/quotaservice/buckets"
	"gopkg.in/redis.v3"
	"github.com/maniksurtani/quotaservice/configs"
	"os"
)

var cfg = configs.NewDefaultServiceConfig()
var factory buckets.BucketFactory
var bucket *redisBucket

func TestMain(m *testing.M) {
	setUp()
	r := m.Run()
	os.Exit(r)
}

func setUp() {
	factory = NewBucketFactory(&redis.Options{Addr: "localhost:6379"}, 2)
	factory.Init(cfg)
	bucket = factory.NewBucket("redis", "redis", configs.NewDefaultBucketConfig(), false).(*redisBucket)
}

func TestScriptLoaded(t *testing.T) {
	if !checkScriptExists(bucket.factory.client, bucket.factory.scriptSHA) {
		t.Fatal("Script not loaded into Redis at start")
	}
}

func TestFailingRedisConn(t *testing.T) {
	w := bucket.Take(1, 0)
	if w != 0 {
		t.Fatalf("Should have not seen any wait time. Saw %v", w)
	}

	err := bucket.factory.client.Close()
	if err != nil {
		t.Fatal("Couldn't kill client.")
	}

	// Client should reconnect
	w = bucket.Take(1, 0)
	if w != 0 {
		t.Fatalf("Should have not seen any wait time. Saw %v", w)
	}
}
