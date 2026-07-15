package rag

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/EsanSamuel/go-hls-transcoder/internal/config"
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

func ProcessChunks(chunks []Chunk, query string) (float32, string, string, error) {
	var queryEmbeddings []float32 = config.AIEmbeddings(query)
	for i := range chunks {
		if chunks[i].Embeddings == nil {
			chunks[i].Embeddings = config.AIEmbeddings(chunks[i].Text)
		}
	}

	for i, chunk := range chunks {
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
	}

	prompt := BuildVideoQAPrompt(query, content)
	answer, err := config.Ai(prompt)
	if err != nil {
		log.Printf("Error generating answer: %v", err)
		return 0, "", "", err
	}
	return chunks[0].Score, answer, prompt, nil
}
