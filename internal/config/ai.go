package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"google.golang.org/genai"
)

func Ai(prompt string) (string, error) {
	ctx := context.Background()

	api_key := os.Getenv("GEMINI_API_KEY")
	config := &genai.ClientConfig{
		APIKey: api_key,
	}
	client, err := genai.NewClient(ctx, config)
	if err != nil {
		log.Fatal(err)
	}

	result, err := client.Models.GenerateContent(
		ctx,
		"gemini-3.5-flash",
		genai.Text(prompt),
		nil,
	)
	if err != nil {
		log.Println(err)
		return "", err
	}
	//fmt.Println(result.Text())
	return result.Text(), nil
}

func AIEmbeddings(text string) []float32 {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}

	contents := []*genai.Content{
		genai.NewContentFromText(text, genai.RoleUser),
	}
	dimensionality := int32(1024)
	result, err := client.Models.EmbedContent(ctx,
		"gemini-embedding-2",
		contents,
		&genai.EmbedContentConfig{OutputDimensionality: &dimensionality},
	)
	if err != nil {
		log.Fatal(err)
	}

	embeddings, err := json.MarshalIndent(result.Embeddings, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(embeddings))
	return result.Embeddings[0].Values
}
