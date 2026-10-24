package clienttest

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockClient struct {
	mock.Mock
}

// NewMockClient registers mock failures and expectation checks with the test.
func NewMockClient(t *testing.T) *MockClient {
	t.Helper()
	m := &MockClient{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}

func (mc *MockClient) Address() string {
	args := mc.Called()
	return args.String(0)
}

func (mc *MockClient) Args() map[string]interface{} {
	args := mc.Called()
	return args.Get(0).(map[string]interface{})
}

func (mc *MockClient) Write(b []byte) (int, error) {
	args := mc.Called(b)
	return args.Int(0), args.Error(1)
}

func (mc *MockClient) Read(b []byte) (int, error) {
	args := mc.Called(b)
	d := args.Get(0).([]byte)
	copy(b, d)
	return len(d), args.Error(1)
}

func (mc *MockClient) Close() error {
	args := mc.Called()
	return args.Error(0)
}

func (mc *MockClient) Key() string {
	args := mc.Called()
	return args.String(0)
}

func LoadData(t *testing.T, fileParts ...string) []byte {
	d, err := ioutil.ReadFile(filepath.Join(fileParts...))
	require.NoError(t, err)
	return d
}
