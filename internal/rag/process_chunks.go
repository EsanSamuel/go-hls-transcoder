package rag

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/EsanSamuel/go-hls-transcoder/internal/config"
	"github.com/EsanSamuel/go-hls-transcoder/internal/database"
	"github.com/pinecone-io/go-pinecone/v4/pinecone"
	"google.golang.org/protobuf/types/known/structpb"
)

const VideoQAInstruction = `You are a retrieval-augmented video QA assistant.
Use only the provided retrieved video chunks and metadata. Do not invent facts.
If the answer is not in the provided content, say you do not have enough information from the video.

The retrieved chunks may include:
- transcript text
- scene descriptions
- spoken dialogue
- detected visual elements
- timestamps
- chapter/segment titles
- audio descriptions
- metadata such as duration, resolution, camera angle, and speakers

When answering:
1. Provide a direct answer first.
2. Reference timestamps, segment names, or chunk labels when available.
3. Note that the answer is based only on the retrieved video data.
4. If multiple chunks conflict, mention the uncertainty and choose the most consistent data.
5. If the question cannot be answered from the retrieved content, respond:
   "I’m sorry, I do not have enough information from the video to answer that."

Example:
Question: "What does the presenter explain at 3 minutes in?"
Answer: "At around 03:00, the presenter explains how the video encoding pipeline works."
Evidence: "Transcript chunk at 03:00-03:15"`

func BuildVideoQAPrompt(query string, retrievedChunks []string) string {
	return fmt.Sprintf(`%s

Query:
%s

Retrieved chunks:
%s`, VideoQAInstruction, query, strings.Join(retrievedChunks, "\n\n"))
}

func retrieveRelevantChunks(databaseClient *pinecone.IndexConnection, queryEmbeddings []float32, topK int) ([]string, float32, error) {
	ctx := context.Background()
	res, err := databaseClient.QueryByVectorValues(ctx, &pinecone.QueryByVectorValuesRequest{
		Vector:          queryEmbeddings,
		TopK:            uint32(topK),
		IncludeMetadata: true,
	})
	if err != nil {
		return nil, 0, err
	}

	var chunks []string
	for _, match := range res.Matches {
		content := match.Vector.Metadata.Fields["content"].GetStringValue()
		chunks = append(chunks, content)
	}
	return chunks, res.Matches[0].Score, nil
}

func ProcessChunks(chunks []Chunk, query string) (float32, string, string, error) {
	databaseClient, err := database.InitPinecone()
	if err != nil {
		log.Printf("Error initializing Pinecone client: %v", err)
		return 0, "", "", err
	}
	fmt.Println("Pinecone Client initialized:", databaseClient)
	var queryEmbeddings []float32 = config.AIEmbeddings(query)
	ctx := context.Background()

	for i := range chunks {
		if chunks[i].Embeddings == nil {
			chunks[i].Embeddings = config.AIEmbeddings(chunks[i].Text)
			metadata, _ := structpb.NewStruct(map[string]interface{}{
				"content": chunks[i].Text,
			})

			vector := &pinecone.Vector{
				Id:       strconv.Itoa(i),
				Values:   &chunks[i].Embeddings,
				Metadata: metadata,
			}

			_, err = databaseClient.UpsertVectors(ctx, []*pinecone.Vector{vector})
		}
	}

	res, score, err := retrieveRelevantChunks(databaseClient, queryEmbeddings, 3)
	if err != nil {
		log.Printf("Error retrieving relevant chunks: %v", err)
		return 0, "", "", err
	}
	fmt.Printf("Results: %v\n", res)
	fmt.Printf("Score: %v\n", score)

	prompt := BuildVideoQAPrompt(query, res)
	answer, err := config.Ai(prompt)
	if err != nil {
		log.Printf("Error generating answer: %v", err)
		return 0, "", "", err
	}
	return score, answer, prompt, nil
}

/*for i, chunk := range chunks {
	score, err := cosineSimilarity(queryEmbeddings, chunk.Embeddings)
	if err != nil {
		log.Printf("Error calculating cosine similarity for chunk %d: %v", i, err)
		continue
	}
	chunks[i].Score = float32(score)
}

sort.Slice(chunks, func(i, j int) bool {
	return chunks[i].Score > chunks[j].Score
})

var scores []float32
for i := range chunks {
	scores = append(scores, chunks[i].Score)
}

sort.Slice(scores, func(i, j int) bool {
	return scores[i] > scores[j]
})

topK := 3
var content []string
for i, chunk := range chunks {
	if i >= topK {
		break
	}
	if i < topK && chunk.Score > 0.35 {
		content = append(content, chunk.Text)
	}
}*/
