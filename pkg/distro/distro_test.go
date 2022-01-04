package distro

import (
	"context"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Distro_New_Alias(t *testing.T) {
	dist, err := New(context.TODO(), "sift", nil, false, "")
	assert.NoError(t, err)

	assert.GreaterOrEqual(t, len(dist.releases), 1)
}

func Test_Distro_New(t *testing.T) {
	dist, err := New(context.TODO(), "ekristen/example-distro-saltstack", nil, false, "")
	assert.NoError(t, err)

	assert.GreaterOrEqual(t, len(dist.releases), 1)
}

func Test_Distro_New_InvalidFormat(t *testing.T) {
	_, err := New(context.TODO(), "example-distro-saltstack", nil, false, "")
	assert.Error(t, err)
}

func Test_Distro_New_SpecifiedVersion(t *testing.T) {
	v := "v1.0.0"
	_, err := New(context.TODO(), "ekristen/example-distro-saltstack", &v, false, "")
	assert.NoError(t, err)
}

func Test_Distro_Manifest_V1_Complete(t *testing.T) {
	dir, err := ioutil.TempDir(os.TempDir(), "distro-")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	os.MkdirAll(dir, 0755)

	dist, err := New(context.TODO(), "sift", nil, false, "")
	assert.NoError(t, err)

	assert.Greater(t, len(dist.releases), 1)

	err = dist.DownloadAssets(dir)
	assert.NoError(t, err)

	err = dist.ValidateAssets(dir)
	assert.NoError(t, err)
}

func Test_Distro_Manifest_V2_Complete(t *testing.T) {
	dir, err := ioutil.TempDir(os.TempDir(), "distro-")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	os.MkdirAll(dir, 0755)

	dist, err := New(context.TODO(), "ekristen/example-distro-saltstack", nil, false, "")
	assert.NoError(t, err)

	assert.GreaterOrEqual(t, len(dist.releases), 1)

	err = dist.DownloadAssets(dir)
	assert.NoError(t, err)

	err = dist.ValidateAssets(dir)
	assert.NoError(t, err)

	err = dist.ExtractArchiveFile(dir)
	assert.NoError(t, err)
}
