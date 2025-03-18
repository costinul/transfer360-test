package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/google/uuid"
)

func checkVehicle(client *pubsub.Client, ctx context.Context, vrm string, company string) error {
	var contravention *VehicleContravention
	var err error

	log.Printf("Checking vehicle: %s, %s\n", vrm, company)

	datasource := getDataSource(company)

	if datasource == nil {
		contravention, err = findContravention(vrm)

		if err != nil {
			return err
		}
	} else {
		contravention, err = SearchContravention(datasource, vrm, time.Now())
		if err != nil {
			if os.IsTimeout(err) {
				log.Printf("Timeout searching for %s in %s\n", vrm, company)
				return nil
			}
			return err
		}
	}

	if contravention == nil || !contravention.IsHirerVehicle {
		log.Printf("Not a hirer vehicle: %s\n", contravention.VRM)
		return nil
	}

	err = sendToPubSub(client, ctx, contravention)

	return err
}

func findContravention(vrm string) (*VehicleContravention, error) {
	for _, datasource := range dataSources {
		contravention, err := SearchContravention(datasource, vrm, time.Now())
		if err != nil {
			if os.IsTimeout(err) {
				log.Printf("Timeout searching for %s in %s\n", vrm, datasource.ID())
				continue
			}
			return nil, err
		}

		if contravention != nil && contravention.IsHirerVehicle {
			return contravention, nil
		}
	}

	return nil, nil
}

func processBatchFile(client *pubsub.Client, ctx context.Context, filePath string) error {
	log.Printf("Processing batch file: %s\n", filePath)
	requests := make([]SearchRequest, 0)

	fileBody, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	err = json.Unmarshal(fileBody, &requests)
	if err != nil {
		return err
	}

	for _, request := range requests {
		err := checkVehicle(client, ctx, request.VRM, request.Company)
		if err != nil {
			return err
		}
	}
	return nil
}

func sendToPubSub(client *pubsub.Client, ctx context.Context, contravention *VehicleContravention) error {
	log.Printf("Sending to pubsub: %s\n", contravention.VRM)
	contravention.Reference = uuid.New().String()

	messageData, err := json.Marshal(contravention)
	if err != nil {
		return err
	}

	topic := client.Topic("positive_searches")
	result := topic.Publish(ctx, &pubsub.Message{
		Data: messageData,
	})

	_, err = result.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed to publish message: %v", err)
	}

	log.Printf("published vrm %s\n", contravention.VRM)
	return nil
}
