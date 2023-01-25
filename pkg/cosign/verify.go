package cosign

import (
	"context"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli/options"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli/verify"
)

func Verify(ctx context.Context, keyPath, sigPath, blobPath string) error {
	ko := options.KeyOpts{
		KeyRef: keyPath,
	}
	verifyBlobCmd := &verify.VerifyBlobCmd{
		KeyOpts:        ko,
		SigRef:         sigPath,
		Offline:        true,
		SkipTlogVerify: true,
	}

	if err := verifyBlobCmd.Exec(ctx, blobPath); err != nil {
		return err
	}

	return nil
}
