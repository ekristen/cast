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

func Test_Distro_New_Alias(t *testing.T) {
	dist, err := NewGitHub(context.TODO(), "sift", nil, false, "", testData)
	assert.NoError(t, err)

	assert.GreaterOrEqual(t, len(dist.(*GitHubDistro).releases), 1)
}

func Test_Distro_NewGitHub(t *testing.T) {
	dist, err := NewGitHub(context.TODO(), "ekristen/example-distro-saltstack", nil, false, "", testData)
	assert.NoError(t, err)

	assert.GreaterOrEqual(t, len(dist.(*GitHubDistro).releases), 1)
}

func Test_Distro_New_InvalidFormat(t *testing.T) {
	_, err := NewGitHub(context.TODO(), "example-distro-saltstack", nil, false, "", testData)
	assert.Error(t, err)
}

func Test_Distro_New_SpecifiedVersion(t *testing.T) {
	v := "v1.0.0"
	_, err := NewGitHub(context.TODO(), "ekristen/example-distro-saltstack", &v, false, "", testData)
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

	dist, err := NewGitHub(context.TODO(), "sift", nil, false, "", testData)
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

	dist, err := NewGitHub(context.TODO(), "ekristen/example-distro-saltstack", nil, false, "", testData)
	assert.NoError(t, err)

	assert.GreaterOrEqual(t, len(dist.(*GitHubDistro).releases), 1)

	err = dist.Download(dir)
	assert.NoError(t, err)
}
