package saltstack

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Installer_installBinary(t *testing.T) {
	dir, err := ioutil.TempDir(os.TempDir(), "edm-")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	cfg := NewConfig()
	cfg.Path = filepath.Join(dir, "saltstack")

	inst := New(cfg)

	err1 := inst.Run()
	assert.NoError(t, err1)

	tarfile := filepath.Join(cfg.Path, "saltstack-binary.tar.gz")
	_, err2 := exists(tarfile)
	assert.NoError(t, err2)

	binfile := filepath.Join(cfg.Path, "salt")
	_, err3 := exists(binfile)
	assert.NoError(t, err3)

	sigfile := filepath.Join(cfg.Path, "saltstack-binary.tar.gz.sha512.asc")
	_, err4 := exists(sigfile)
	assert.NoError(t, err4)

	binpath := inst.GetBinary()
	assert.Equal(t, filepath.Join(cfg.Path, "salt"), binpath)
}

func exists(name string) (bool, error) {
	_, err := os.Stat(name)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}
