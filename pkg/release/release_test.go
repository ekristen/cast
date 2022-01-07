package release

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Run(t *testing.T) {
	cwd, temp := setup(t)
	defer teardown(t, cwd, temp)

}

func setup(t *testing.T) (string, string) {
	cwd, err := os.Getwd()
	require.NoError(t, err)

	temp, err := ioutil.TempDir(os.TempDir(), "rel-")
	require.NoError(t, err)

	err = os.Chdir(temp)
	require.NoError(t, err)

	return cwd, temp
}

func teardown(t *testing.T, cwd, temp string) {
	err := os.Chdir(cwd)
	require.NoError(t, err)

	err = os.RemoveAll(temp)
	require.NoError(t, err)
}
