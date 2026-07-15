package database

import (
	"context"
	//"log"
	"os"

	"github.com/pinecone-io/go-pinecone/v4/pinecone"
)

var idxConn *pinecone.IndexConnection

func InitPinecone() (*pinecone.IndexConnection, error) {
	ctx := context.Background()
	pc, err := pinecone.NewClient(pinecone.NewClientParams{
		ApiKey: os.Getenv("PINECONE_API_KEY"),
	})
	if err != nil {
		return nil, err
	}

	idx, err := pc.DescribeIndex(ctx, "vod")
	if err != nil {
		return nil, err
	}

	idxConn, err = pc.Index(pinecone.NewIndexConnParams{
		Host:      idx.Host,
		Namespace: "docs",
	})
	return idxConn, err
}
