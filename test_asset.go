package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"cloud.google.com/go/asset/apiv1"
	"cloud.google.com/go/asset/apiv1/assetpb"
	"google.golang.org/api/iterator"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := asset.NewClient(ctx)
	if err != nil {
		log.Fatalf("Error creating client: %v", err)
	}
	defer client.Close()

	req := &assetpb.ListAssetsRequest{
		Parent: "projects/development-485208",
	}

	it := client.ListAssets(ctx, req)
	for {
		a, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Println(a.Name)
	}
}
