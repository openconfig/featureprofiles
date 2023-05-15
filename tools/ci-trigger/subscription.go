// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"encoding/json"
	"fmt"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"github.com/golang/glog"
)

type badgeState struct {
	Path   string
	Status string
}

// updateBadgeStatus updates the badgeState.Path in the gcpBucket
// to the new status value.  It requires the object to already exist
// and maintains the previous metadata values.
func updateBadgeStatus(ctx context.Context, bs *badgeState) error {
	storClient, err := storage.NewClient(ctx)
	if err != nil {
		return err
	}
	objAttrs, err := storClient.Bucket(gcpBucket).Object(bs.Path).Attrs(ctx)
	if err != nil {
		return err
	}

	label, ok := objAttrs.Metadata["label"]
	if !ok {
		return fmt.Errorf("object %s missing metadata label", bs.Path)
	}

	buf, err := svgBadge(label, bs.Status)
	if err != nil {
		return err
	}

	obj := storClient.Bucket(gcpBucket).Object(bs.Path).NewWriter(ctx)
	obj.ContentType = objAttrs.ContentType
	obj.CacheControl = objAttrs.CacheControl
	obj.Metadata = objAttrs.Metadata
	obj.Metadata["status"] = bs.Status
	if _, err := buf.WriteTo(obj); err != nil {
		return err
	}

	return obj.Close()
}

// pullSubscription subscribes to the gcpBadgeTopic on gcpProjectID and
// processes the messages.
func pullSubscription() {
	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, gcpProjectID)
	if err != nil {
		glog.Fatalf("Failed creating pubsub client: %s", err)
	}
	defer client.Close()

	sub := client.Subscription(gcpBadgeTopic)
	err = sub.Receive(ctx, func(_ context.Context, msg *pubsub.Message) {
		msg.Ack()
		bs := &badgeState{}
		if err := json.Unmarshal(msg.Data, bs); err != nil {
			glog.Errorf("Failed to decode subscription message %q: %s", msg.Data, err)
			return
		}
		if err := updateBadgeStatus(ctx, bs); err != nil {
			glog.Errorf("Failed to update badge state: %s", err)
			return
		}
	})
	if err != nil {
		glog.Fatalf("Failed receiving pubsub message: %s", err)
	}
}
