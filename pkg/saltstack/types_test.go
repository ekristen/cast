package saltstack

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ParseLocalResults_SiftSuccessResults(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/sift-success.txt")
	assert.NoError(t, err)

	res, err := ParseLocalResults(data)
	assert.NoError(t, err)

	assert.Len(t, res.Local, 563)
}
