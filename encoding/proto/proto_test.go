package proto

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/Tsukikage7/servex/v2/auth/proto"
	"github.com/Tsukikage7/servex/v2/encoding"
)

// ProtoCodecTestSuite proto 编解码器测试套件.
type ProtoCodecTestSuite struct {
	suite.Suite
}

func TestProtoCodecSuite(t *testing.T) {
	suite.Run(t, new(ProtoCodecTestSuite))
}

func (s *ProtoCodecTestSuite) TestInit_Registered() {
	c := encoding.GetCodec("proto")
	s.NotNil(c)
	s.Equal("proto", c.Name())
}

func (s *ProtoCodecTestSuite) TestMarshal_ProtoMessage() {
	c := codec{}
	msg := &authpb.MethodAuthOptions{
		Public:      true,
		Permissions: []string{"read", "write"},
	}

	data, err := c.Marshal(msg)
	s.NoError(err)
	s.NotEmpty(data)
	// protojson 输出包含字段名
	s.Contains(string(data), "public")
	s.Contains(string(data), "read")
}

func (s *ProtoCodecTestSuite) TestMarshal_NonProtoMessage() {
	c := codec{}
	v := map[string]string{"key": "value"}

	data, err := c.Marshal(v)
	s.NoError(err)
	s.Equal(`{"key":"value"}`, string(data))
}

func (s *ProtoCodecTestSuite) TestUnmarshal_ProtoMessage() {
	c := codec{}
	msg := &authpb.MethodAuthOptions{}
	data := []byte(`{"public":true,"permissions":["admin"]}`)

	err := c.Unmarshal(data, msg)
	s.NoError(err)
	s.True(msg.GetPublic())
	s.Equal([]string{"admin"}, msg.GetPermissions())
}

func (s *ProtoCodecTestSuite) TestUnmarshal_NonProtoMessage() {
	c := codec{}
	var v map[string]string
	data := []byte(`{"key":"value"}`)

	err := c.Unmarshal(data, &v)
	s.NoError(err)
	s.Equal("value", v["key"])
}

func (s *ProtoCodecTestSuite) TestMarshal_ProtoMessage_EmitUnpopulated() {
	c := codec{}
	msg := &authpb.MethodAuthOptions{}

	data, err := c.Marshal(msg)
	s.NoError(err)
	// pbjson.MarshalOptions.EmitUnpopulated = true，应输出零值字段
	s.Contains(string(data), "public")
}

func (s *ProtoCodecTestSuite) TestUnmarshal_ProtoMessage_InvalidJSON() {
	c := codec{}
	msg := &authpb.MethodAuthOptions{}

	err := c.Unmarshal([]byte(`{invalid`), msg)
	s.Error(err)
}

func (s *ProtoCodecTestSuite) TestUnmarshal_NonProtoMessage_InvalidJSON() {
	c := codec{}
	var v map[string]string

	err := c.Unmarshal([]byte(`{invalid`), &v)
	s.Error(err)
}

func (s *ProtoCodecTestSuite) TestName() {
	c := codec{}
	s.Equal("proto", c.Name())
}
