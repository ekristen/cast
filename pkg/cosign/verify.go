package cosign

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/sigstore/cosign/v2/pkg/blob"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	"github.com/sigstore/cosign/v2/pkg/cosign/pkcs11key"
	"github.com/sigstore/cosign/v2/pkg/oci/static"
	sigs "github.com/sigstore/cosign/v2/pkg/signature"
	"github.com/sirupsen/logrus"
	"io"
	"os"
)

func Verify(ctx context.Context, keyRef, sigRef, blobRef string) error {
	/*
		ko := options.KeyOpts{
			KeyRef: keyPath,
		}
		verifyBlobCmd := &verify.VerifyBlobCmd{
			KeyOpts:        ko,
			SigRef:         sigRef,
			Offline:        true,
			SkipTlogVerify: true,
		}

		if err := verifyBlobCmd.Exec(ctx, blobRef); err != nil {
			return err
		}
	*/

	var identities []cosign.Identity
	opts := make([]static.Option, 0)

	sig, err := base64signature(sigRef, "")
	if err != nil {
		return err
	}

	blobBytes, err := payloadBytes(blobRef)
	if err != nil {
		return err
	}

	co := &cosign.CheckOpts{
		IgnoreSCT:  true,
		Identities: identities,
		Offline:    true,
		IgnoreTlog: true,
	}

	co.SigVerifier, err = sigs.PublicKeyFromKeyRef(ctx, keyRef)
	if err != nil {
		return fmt.Errorf("loading public key: %w", err)
	}
	pkcs11Key, ok := co.SigVerifier.(*pkcs11key.Key)
	if ok {
		defer pkcs11Key.Close()
	}

	signature, err := static.NewSignature(blobBytes, sig, opts...)
	if err != nil {
		return err
	}
	if _, err = cosign.VerifyBlobSignature(ctx, signature, co); err != nil {
		return err
	}

	logrus.WithField("component", "cosign").Info("signatures verified")

	return nil
}

// base64signature returns the base64 encoded signature
func base64signature(sigRef, bundlePath string) (string, error) {
	var targetSig []byte
	var err error

	targetSig, err = blob.LoadFileOrURL(sigRef)
	if err != nil {
		if !os.IsNotExist(err) {
			// ignore if file does not exist, it can be a base64 encoded string as well
			return "", err
		}
		targetSig = []byte(sigRef)
	}

	if isb64(targetSig) {
		return string(targetSig), nil
	}
	return base64.StdEncoding.EncodeToString(targetSig), nil
}

func payloadBytes(blobRef string) ([]byte, error) {
	var blobBytes []byte
	var err error
	if blobRef == "-" {
		blobBytes, err = io.ReadAll(os.Stdin)
	} else {
		blobBytes, err = blob.LoadFileOrURL(blobRef)
	}
	if err != nil {
		return nil, err
	}
	return blobBytes, nil
}

func isb64(data []byte) bool {
	_, err := base64.StdEncoding.DecodeString(string(data))
	return err == nil
}
