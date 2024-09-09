package mongodb

import (
	"context"
	"crypto/tls"

	"go.mongodb.org/mongo-driver/mongo"
	mongooptions "go.mongodb.org/mongo-driver/mongo/options"
)

type ClientOptions struct {
	Hosts              []string
	Username           string
	Password           string
	AuthSource         string
	ReplicaSet         string
	TLS                bool
	InsecureSkipVerify bool
}

type Client struct {
	mongo *mongo.Client

	ClientOptions
}

func New(ctx context.Context, options *ClientOptions) (*Client, error) {
	opt := mongooptions.Client().
		SetHosts(options.Hosts).
		SetAuth(mongooptions.Credential{
			Username:   options.Username,
			Password:   options.Password,
			AuthSource: options.AuthSource,
		}).
		SetReplicaSet(options.ReplicaSet)

	if options.TLS {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: options.InsecureSkipVerify,
		}

		opt.SetTLSConfig(tlsConfig)
	}

	mongoClient, err := mongo.Connect(ctx, opt)
	if err != nil {
		return nil, err
	}

	err = mongoClient.Ping(ctx, nil)
	if err != nil {
		return nil, err
	}

	client := &Client{
		mongo:         mongoClient,
		ClientOptions: *options,
	}

	return client, nil
}
