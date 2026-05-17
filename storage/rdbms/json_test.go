package rdbms

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type JsonValueTestSuite struct {
	suite.Suite
}

func TestJsonValueSuite(t *testing.T) {
	suite.Run(t, new(JsonValueTestSuite))
}

func (s *JsonValueTestSuite) TestValue() {
	jv := NewJsonValue(address{City: "Hangzhou", Country: "CN"})

	val, err := jv.Value()

	s.NoError(err)
	s.Contains(val.(string), `"city":"Hangzhou"`)
}

func (s *JsonValueTestSuite) TestScanNilUsesZeroValue() {
	jv := NewJsonValue(address{City: "old", Country: "CN"})

	err := jv.Scan(nil)

	s.NoError(err)
	s.Equal(address{}, jv.Val)
}

func (s *JsonValueTestSuite) TestRoundTripSlice() {
	original := NewJsonValue([]string{"a", "b"})
	val, err := original.Value()
	s.NoError(err)

	var restored JsonValue[[]string]
	err = restored.Scan(val)

	s.NoError(err)
	s.Equal([]string{"a", "b"}, restored.Val)
}

func (s *JsonValueTestSuite) TestScanUnsupportedType() {
	var jv JsonValue[address]

	err := jv.Scan(42)

	s.Error(err)
}

func (s *JsonValueTestSuite) TestJsonSliceNilValueUsesEmptyArray() {
	var js JsonSlice[string]

	val, err := js.Value()

	s.NoError(err)
	s.Equal("[]", val)
}

func (s *JsonValueTestSuite) TestJsonSliceScanNilUsesEmptySlice() {
	js := JsonSlice[string]{"old"}

	err := js.Scan(nil)

	s.NoError(err)
	s.NotNil(js)
	s.Empty(js)
}

func (s *JsonValueTestSuite) TestJsonSliceRoundTrip() {
	original := JsonSlice[address]{{City: "Shenzhen", Country: "CN"}}
	val, err := original.Value()
	s.NoError(err)

	var restored JsonSlice[address]
	err = restored.Scan(val)

	s.NoError(err)
	s.Equal(original, restored)
}
