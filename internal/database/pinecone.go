package database

import (
	"log"
	"os"

	"github.com/pinecone-io/go-pinecone/v4/pinecone"
)

func PineconeClient() *pinecone.Client {
	apiKey := os.Getenv("PINECONE_API_KEY")

	pc, err := pinecone.NewClient(pinecone.NewClientParams{
		ApiKey: apiKey,
	})
	if err != nil {
		log.Printf("Failed to create Client: %v", err)
	}
	return pc
}
