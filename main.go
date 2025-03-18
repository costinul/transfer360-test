package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"cloud.google.com/go/pubsub"
	"google.golang.org/api/option"
)

type ClientFactory struct {
	projectID string
	opts      []option.ClientOption
}

type VehicleRegistrationRequest struct {
	VRM     string `json:"vrm"`
	Company string `json:"company"`
}

var clientFactory *ClientFactory

func (f *ClientFactory) CreateClient(ctx context.Context) (*pubsub.Client, error) {
	return pubsub.NewClient(ctx, f.projectID, f.opts...)
}

type Flags struct {
	ProjectID   string
	UseEmulator bool
	CredFile    string
	VRM         string
	Company     string
	BatchFile   string
}

func parseAndValidateFlags() (*Flags, error) {
	projectID := flag.String("project", "", "Google Cloud Project ID (required)")
	useEmulator := flag.Bool("emulator", false, "Use Pub/Sub emulator")
	credFile := flag.String("creds", "", "Path to service account credentials JSON file")
	vrm := flag.String("vrm", "", "Vehicle Registration Mark")
	company := flag.String("company", "", "Company name")
	batchFile := flag.String("batch", "", "File containing VRM and company pairs")

	flag.Parse()

	if *projectID == "" {
		return nil, fmt.Errorf("missing required flag: -project (required for both emulator and production)")
	}

	if *batchFile != "" {
		if *vrm != "" || *company != "" {
			return nil, fmt.Errorf("batch file cannot be used together with VRM or company flags")
		}
		if _, err := os.Stat(*batchFile); os.IsNotExist(err) {
			return nil, fmt.Errorf("batch file does not exist: %s", *batchFile)
		}
	} else if *company != "" && *vrm == "" {
		return nil, fmt.Errorf("company flag requires VRM flag to be set")
	}

	return &Flags{
		ProjectID:   *projectID,
		UseEmulator: *useEmulator,
		CredFile:    *credFile,
		VRM:         *vrm,
		Company:     *company,
		BatchFile:   *batchFile,
	}, nil
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("Error: %v", err)
		os.Exit(1)
	}
}

func run() error {
	var emulator *PubSubEmulator
	flags, err := parseAndValidateFlags()
	if err != nil {
		return err
	}

	if flags.UseEmulator {
		log.Printf("Using emulator with project ID: %s (can be any string when using emulator)", flags.ProjectID)
	}

	var opts []option.ClientOption

	// Create main context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if flags.UseEmulator {
		emulator = NewPubSubEmulator(flags.ProjectID, 8085)
		err := emulator.Start(ctx)
		if err != nil {
			return fmt.Errorf("failed to start emulator: %v", err)
		}

		defer emulator.Stop()
		opts = append(opts, option.WithEndpoint(emulator.Host()))
		opts = append(opts, option.WithoutAuthentication())
	} else if flags.CredFile != "" {
		log.Printf("Using service account credentials from: %s", flags.CredFile)
		opts = append(opts, option.WithCredentialsFile(flags.CredFile))
	}

	clientFactory = &ClientFactory{
		projectID: flags.ProjectID,
		opts:      opts,
	}

	err = createTopic(ctx, "positive_searches")
	if err != nil {
		return fmt.Errorf("failed to create topic: %v", err)
	}
	initDataSources()

	client, err := clientFactory.CreateClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create pubsub client: %v", err)
	}
	defer client.Close()

	if flags.BatchFile != "" {
		err := processBatchFile(client, ctx, flags.BatchFile)
		if err != nil {
			return fmt.Errorf("failed to process batch file: %v", err)
		}
	} else if flags.VRM != "" && flags.Company != "" {
		err := checkVehicle(client, ctx, flags.VRM, flags.Company)
		if err != nil {
			return fmt.Errorf("failed to check vehicle: %v", err)
		}
	}

	if flags.UseEmulator {
		fmt.Println("\nPress Enter to stop emulator...")
		bufio.NewReader(os.Stdin).ReadBytes('\n')
	}

	return nil
}

func createTopic(ctx context.Context, topicName string) error {
	client, err := clientFactory.CreateClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create pubsub client: %v", err)
	}

	topic := client.Topic(topicName)
	exists, err := topic.Exists(ctx)
	if err != nil {
		return err
	}

	if !exists {
		_, err = client.CreateTopic(ctx, topicName)
		if err != nil {
			return err
		}
	}

	return nil
}
