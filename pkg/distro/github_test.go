package distro

import (
	"context"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

var testData = map[string]string{
	"User": "test_user",
}

// These tests pass skipValidation, because the operating system check depends
// on the host running the tests. The SIFT alias, for one, only supports Ubuntu
// 20.04.

func Test_Distro_New_Alias(t *testing.T) {
	dist, err := NewGitHub(context.TODO(), "sift", nil, true, false, "", testData)
	assert.NoError(t, err)

	assert.GreaterOrEqual(t, len(dist.(*GitHubDistro).releases), 1)
}

func Test_Distro_NewGitHub(t *testing.T) {
	dist, err := NewGitHub(context.TODO(), "ekristen/example-distro-saltstack", nil, true, false, "", testData)
	assert.NoError(t, err)

	assert.GreaterOrEqual(t, len(dist.(*GitHubDistro).releases), 1)
}

func Test_Distro_New_InvalidFormat(t *testing.T) {
	_, err := NewGitHub(context.TODO(), "example-distro-saltstack", nil, true, false, "", testData)
	assert.Error(t, err)
}

func Test_Distro_New_SpecifiedVersion(t *testing.T) {
	v := "v1.3.0" // v1.0.0 to v1.2.0 shipped manifest.yaml, which cast never read
	_, err := NewGitHub(context.TODO(), "ekristen/example-distro-saltstack", &v, true, false, "", testData)
	assert.NoError(t, err)
}

func Test_Distro_Manifest_V1_Complete(t *testing.T) {
	dir, err := ioutil.TempDir(os.TempDir(), "distro-")
	assert.NoError(t, err)
	defer func(path string) {
		_ = os.RemoveAll(path)
	}(dir)

	err1 := os.MkdirAll(dir, 0755)
	assert.NoError(t, err1)

	dist, err := NewGitHub(context.TODO(), "sift", nil, true, false, "", testData)
	assert.NoError(t, err)

	assert.Greater(t, len(dist.(*GitHubDistro).releases), 1)

	err = dist.Download(dir)
	assert.NoError(t, err)
}

func Test_Distro_Manifest_V2_Complete(t *testing.T) {
	dir, err := ioutil.TempDir(os.TempDir(), "distro-")
	assert.NoError(t, err)
	defer func(path string) {
		_ = os.RemoveAll(path)
	}(dir)

	err1 := os.MkdirAll(dir, 0755)
	assert.NoError(t, err1)

	dist, err := NewGitHub(context.TODO(), "ekristen/example-distro-saltstack", nil, true, false, "", testData)
	assert.NoError(t, err)

	assert.GreaterOrEqual(t, len(dist.(*GitHubDistro).releases), 1)

	err = dist.Download(dir)
	assert.NoError(t, err)
}
