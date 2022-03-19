package utils

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/ProtonMail/gopenpgp/v2/helper"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/armor"
	"golang.org/x/crypto/openpgp/packet"
)

func GPGVerify(dir, filename, checksumFilename string, pgpPublicKey []byte) error {
	log := logrus.WithField("filename", filename)
	log.Info("validating file pgp signature")

	filename = filepath.Join(dir, filename)

	if exists, err := FileExists(fmt.Sprintf("%s.valid", filename)); err != nil {
		return err
	} else if exists {
		return nil
	}

	fileContent, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}

	// Get a Reader for the signature file
	sigFile, err := os.Open(filepath.Join(dir, checksumFilename))
	if err != nil {
		return err
	}
	defer sigFile.Close()

	defer func() {
		if err := sigFile.Close(); err != nil {
			panic(err)
		}
	}()

	block, err := armor.Decode(sigFile)
	if err != nil {
		return fmt.Errorf("error decoding signature file: %s", err)
	}
	if block.Type != "PGP SIGNATURE" {
		return errors.New("not an armored signature or message")
	}

	// Read the signature file
	pack, err := packet.Read(block.Body)
	if err != nil {
		return err
	}

	// Was it really a signature file ? If yes, get the Signature
	signature, ok := pack.(*packet.Signature)
	if !ok {
		return errors.New("not a valid signature file")
	}

	block, err = armor.Decode(bytes.NewReader(pgpPublicKey))
	if err != nil {
		return fmt.Errorf("error decoding public key: %s", err)
	}
	if block.Type != "PGP PUBLIC KEY BLOCK" {
		return errors.New("not an armored public key")
	}

	// Read the key
	pack, err = packet.Read(block.Body)
	if err != nil {
		return fmt.Errorf("error reading public key: %s", err)
	}

	// Was it really a public key file ? If yes, get the PublicKey
	publicKey, ok := pack.(*packet.PublicKey)
	if !ok {
		return errors.New("invalid public key")
	}

	// Get the hash method used for the signature
	hash := signature.Hash.New()

	// Hash the content of the file (if the file is big, that's where you have to change the code to avoid getting the whole file in memory, by reading and writting in small chunks)
	_, err = hash.Write(fileContent)
	if err != nil {
		return err
	}

	// Check the signature
	err = publicKey.VerifySignature(hash, signature)
	if err != nil {
		return err
	}

	return nil
}

func GPGSign(dir, filename, signatureFile string, pgpPrivateKey []byte, detached bool) error {
	filename = filepath.Join(dir, filename)
	signatureFile = filepath.Join(dir, signatureFile)

	block, err := armor.Decode(bytes.NewReader(pgpPrivateKey))
	if err != nil {
		return fmt.Errorf("error decoding public key: %s", err)
	}

	if block.Type != "PGP PRIVATE KEY BLOCK" {
		return errors.New("not an armored public key")
	}

	pack, err := packet.Read(block.Body)
	if err != nil {
		return fmt.Errorf("error reading public key: %s", err)
	}

	privateKey, ok := pack.(*packet.PrivateKey)
	if !ok {
		return errors.New("invalid public key")
	}

	w, err := os.Create(signatureFile)
	if err != nil {
		return err
	}
	defer w.Close()

	if detached {
		message, err := os.Open(filename)
		if err != nil {
			return err
		}
		defer message.Close()

		if err := openpgp.ArmoredDetachSignText(w, &openpgp.Entity{
			PrimaryKey: &privateKey.PublicKey,
			PrivateKey: privateKey,
		}, message, nil); err != nil {
			logrus.WithError(err).Error("unable to sign armored detached")
			return err
		}
		w.Write([]byte("\n"))
	} else {
		message, err := ioutil.ReadFile(filename)
		if err != nil {
			return err
		}

		signingKey, err := crypto.NewKeyFromArmored(string(pgpPrivateKey))
		if err != nil {
			return err
		}

		keyRing, err := crypto.NewKeyRing(signingKey)
		if err != nil {
			return err
		}

		armored, err := helper.SignCleartextMessage(keyRing, string(message))
		if err != nil {
			return err
		}
		w.Write([]byte(armored))
		w.Write([]byte("\n"))
	}

	return nil
}
