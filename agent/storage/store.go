package storage

import "context"

type Store interface {
	Put(ctx context.Context, key string, data []byte, opts PutOptions) error
}

type PutOptions struct {
	ObjectLock    bool
	RetentionDays int
	ContentType   string
}
