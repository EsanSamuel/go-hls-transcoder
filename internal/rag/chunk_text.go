package rag

import "strings"

type Chunk struct {
	ChunkID    int
	Text       string
	Embeddings []float32
	Score      float32
}

func ChunkText(text string) []Chunk {
	const (
		chunkSize = 250
		overlap   = 50
	)

	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	var chunks []Chunk
	step := chunkSize - overlap

	chunkID := 1

	for i := 0; i < len(words); i += step {
		end := i + chunkSize
		if end > len(words) {
			end = len(words)
		}

		chunks = append(chunks, Chunk{
			ChunkID: chunkID,
			Text:    strings.Join(words[i:end], " "),
		})

		chunkID++

		if end == len(words) {
			break
		}
	}

	return chunks
}
