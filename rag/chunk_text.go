package rag

import "strings"

type Chunk struct {
	ChunkID    int
	Text       string
	Embeddings []float32
	Score      float32
}

func ChunkText(text string) []Chunk {
	paragraphs := strings.Split(text, "\n")
	chunkID := 0
	var Chunks []Chunk

	for _, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)
		Chunks = append(Chunks, Chunk{
			Text:    paragraph,
			ChunkID: chunkID + 1,
		})
		chunkID++
	}
	return Chunks
}
