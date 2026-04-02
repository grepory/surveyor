package signing

import "context"

type Signer interface {
	Sign(ctx context.Context, declaration []byte) (SignedResult, error)
}

type SignedResult struct {
	Signature     []byte
	Certificate   []byte // keyless only
	RekorLogEntry string // empty for BYOK without Rekor
	ContentHash   string
}
