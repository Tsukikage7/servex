package rdbms

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type EncryptColumnTestSuite struct {
	suite.Suite
}

func TestEncryptColumnSuite(t *testing.T) {
	suite.Run(t, new(EncryptColumnTestSuite))
}

// AES-256 需要 32 字节密钥.
const testKey = "01234567890123456789012345678901"

func (s *EncryptColumnTestSuite) TestNewEncryptColumn() {
	ec, err := NewEncryptColumn("secret-ssn", testKey)
	s.NoError(err)
	s.True(ec.Valid)
	s.Equal("secret-ssn", ec.Val)
	s.Equal(testKey, ec.GetKey())
}

func (s *EncryptColumnTestSuite) TestNewEncryptColumn_InvalidKeyLength() {
	_, err := NewEncryptColumn("data", "short-key")
	s.Error(err)
	s.Contains(err.Error(), "无效的 AES 密钥长度")
}

func (s *EncryptColumnTestSuite) TestNullEncryptColumn() {
	ec, err := NullEncryptColumn[string](testKey)
	s.NoError(err)
	s.False(ec.Valid)
}

func (s *EncryptColumnTestSuite) TestNullEncryptColumn_EmptyKey() {
	// 空 key 允许（延迟注入密钥场景）
	ec, err := NullEncryptColumn[string]("")
	s.NoError(err)
	s.False(ec.Valid)
}

func (s *EncryptColumnTestSuite) TestNullEncryptColumn_InvalidKeyLength() {
	_, err := NullEncryptColumn[string]("short-key")
	s.Error(err)
	s.Contains(err.Error(), "无效的 AES 密钥长度")
}

func (s *EncryptColumnTestSuite) TestValue_Valid() {
	ec, err := NewEncryptColumn("hello", testKey)
	s.NoError(err)
	val, err := ec.Value()
	s.NoError(err)
	s.NotNil(val)
	s.IsType("", val)
}

func (s *EncryptColumnTestSuite) TestValue_Null() {
	ec, err := NullEncryptColumn[string](testKey)
	s.NoError(err)
	val, err := ec.Value()
	s.NoError(err)
	s.Nil(val)
}

func (s *EncryptColumnTestSuite) TestRoundTrip_String() {
	original, err := NewEncryptColumn("sensitive-data-123", testKey)
	s.NoError(err)

	val, err := original.Value()
	s.NoError(err)

	restored, err := NullEncryptColumn[string](testKey)
	s.NoError(err)
	err = restored.Scan(val)
	s.NoError(err)
	s.True(restored.Valid)
	s.Equal("sensitive-data-123", restored.Val)
}

func (s *EncryptColumnTestSuite) TestRoundTrip_Struct() {
	type secret struct {
		SSN  string `json:"ssn"`
		Code int    `json:"code"`
	}

	original, err := NewEncryptColumn(secret{SSN: "123-45-6789", Code: 42}, testKey)
	s.NoError(err)

	val, err := original.Value()
	s.NoError(err)

	restored, err := NullEncryptColumn[secret](testKey)
	s.NoError(err)
	err = restored.Scan(val)
	s.NoError(err)
	s.True(restored.Valid)
	s.Equal("123-45-6789", restored.Val.SSN)
	s.Equal(42, restored.Val.Code)
}

func (s *EncryptColumnTestSuite) TestScan_Nil() {
	ec, err := NewEncryptColumn("old", testKey)
	s.NoError(err)
	err = ec.Scan(nil)
	s.NoError(err)
	s.False(ec.Valid)
}

func (s *EncryptColumnTestSuite) TestScan_UnsupportedType() {
	ec, err := NullEncryptColumn[string](testKey)
	s.NoError(err)
	err = ec.Scan(42)
	s.Error(err)
}

func (s *EncryptColumnTestSuite) TestScan_WithoutKey() {
	original, err := NewEncryptColumn("data", testKey)
	s.NoError(err)
	val, err := original.Value()
	s.NoError(err)

	noKey, err := NullEncryptColumn[string]("")
	s.NoError(err)
	err = noKey.Scan(val)
	s.NoError(err)
	s.False(noKey.Valid)
	s.NotEmpty(noKey.Ciphertext)
}

func (s *EncryptColumnTestSuite) TestDecrypt_AfterScanWithoutKey() {
	original, err := NewEncryptColumn("secret-data", testKey)
	s.NoError(err)
	val, err := original.Value()
	s.NoError(err)

	ec, err := NullEncryptColumn[string]("")
	s.NoError(err)
	err = ec.Scan(val)
	s.NoError(err)
	s.False(ec.Valid)

	ec.SetKey(testKey)
	err = ec.Decrypt()
	s.NoError(err)
	s.True(ec.Valid)
	s.Equal("secret-data", ec.Val)
	s.Empty(ec.Ciphertext)
}

func (s *EncryptColumnTestSuite) TestDecrypt_AlreadyValid() {
	ec, err := NewEncryptColumn("data", testKey)
	s.NoError(err)
	err = ec.Decrypt()
	s.NoError(err)
}

func (s *EncryptColumnTestSuite) TestDecrypt_NoKey() {
	ec := &EncryptColumn[string]{Ciphertext: "something"}
	err := ec.Decrypt()
	s.Error(err)
}

func (s *EncryptColumnTestSuite) TestDecrypt_NoCiphertext() {
	ec := &EncryptColumn[string]{}
	ec.SetKey(testKey)
	err := ec.Decrypt()
	s.Error(err)
}

func (s *EncryptColumnTestSuite) TestScan_WrongKey() {
	original, err := NewEncryptColumn("data", testKey)
	s.NoError(err)
	val, err := original.Value()
	s.NoError(err)

	wrongKey := "99999999999999999999999999999999"
	restored, err := NullEncryptColumn[string](wrongKey)
	s.NoError(err)
	err = restored.Scan(val)
	s.Error(err)
}

func (s *EncryptColumnTestSuite) TestScan_InvalidBase64() {
	ec, err := NullEncryptColumn[string](testKey)
	s.NoError(err)
	err = ec.Scan("not-valid-base64!!!")
	s.Error(err)
}

func (s *EncryptColumnTestSuite) TestScan_ByteInput() {
	original, err := NewEncryptColumn("test", testKey)
	s.NoError(err)
	val, err := original.Value()
	s.NoError(err)

	restored, err := NullEncryptColumn[string](testKey)
	s.NoError(err)
	err = restored.Scan([]byte(val.(string)))
	s.NoError(err)
	s.True(restored.Valid)
	s.Equal("test", restored.Val)
}

func (s *EncryptColumnTestSuite) TestValue_InvalidKey() {
	// 构造时就应该校验密钥长度
	_, err := NewEncryptColumn("data", "short-key")
	s.Error(err)
}

func (s *EncryptColumnTestSuite) TestDifferentEncryptions() {
	// 相同数据加密两次，密文应不同（随机 nonce）
	ec1, err := NewEncryptColumn("same-data", testKey)
	s.NoError(err)
	ec2, err := NewEncryptColumn("same-data", testKey)
	s.NoError(err)

	val1, err1 := ec1.Value()
	val2, err2 := ec2.Value()

	s.NoError(err1)
	s.NoError(err2)
	s.NotEqual(val1, val2)
}
