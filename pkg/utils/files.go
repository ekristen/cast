package utils

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"github.com/mholt/archives"
	"github.com/sirupsen/logrus"
	"io"
	"os"
	"path/filepath"
)

func FileExists(name string) (bool, error) {
	_, err := os.Stat(name)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func ExtractTarballGz(file string, dst string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()

		switch {

		// if no more files are found return
		case err == io.EOF:
			return nil

		// return any other error
		case err != nil:
			return err

		// if the header is nil, just skip it (not sure how this happens)
		case header == nil:
			continue
		}

		// the target location where the dir/file should be created
		target := filepath.Join(dst, header.Name)

		// the following switch could also be done using fi.Mode(), not sure if there
		// a benefit of using one vs. the other.
		// fi := header.FileInfo()

		// check the file type
		switch header.Typeflag {

		// if its a dir and it doesn't exist create it
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0755); err != nil {
					return err
				}
			}

		// if it's a file create it
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			// copy over contents
			if _, err := io.Copy(f, tr); err != nil {
				return err
			}

			// manually close here after each file operation; defering would cause each file close
			// to wait until all operations have completed.
			f.Close()
		}
	}
}

func ExtractArchive(file string, dst string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	format, stream, err := archives.Identify(context.TODO(), file, f)
	if err != nil {
		return err
	}

	if ex, ok := format.(archives.Extractor); ok {
		logrus.Debug("extracting archive")
		if err := ex.Extract(context.TODO(), stream, func(ctx context.Context, f archives.FileInfo) error {
			target := filepath.Join(dst, f.NameInArchive)

			if f.Mode().IsDir() {
				if _, err := os.Stat(target); err != nil {
					if err := os.MkdirAll(target, 0755); err != nil {
						return err
					}
					logrus.Tracef("archive > create directory %s", target)
				}

				return nil
			}

			if f.LinkTarget != "" {
				newTarget := filepath.Join(filepath.Dir(target), f.LinkTarget)
				logrus.Tracef("archive > create symlink %s -> %s", newTarget, target)
				return os.Symlink(newTarget, target)
			}

			tc, err := f.Open()
			if err != nil {
				return err
			}
			defer tc.Close()

			nf, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, f.Mode())
			if err != nil {
				return err
			}
			defer nf.Close()

			// copy over contents
			if _, err := io.Copy(nf, tc); err != nil {
				return err
			}

			logrus.Tracef("writing destination file: %s", target)

			return nil
		}); err != nil {
			return err
		}
	} else {
		return errors.New("unsupported archive format")
	}

	return nil
}
