package crypto

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type CryptoTestSuite struct {
	suite.Suite
}

func TestCryptoSuite(t *testing.T) {
	suite.Run(t, new(CryptoTestSuite))
}

func (s *CryptoTestSuite) TestGenerateID() {
	id, err := GenerateID()
	s.Require().NoError(err)
	s.Len(id, 32, "ID 应为 32 位十六进制字符串")

	// 每次生成的 ID 应不同
	id2, err := GenerateID()
	s.Require().NoError(err)
	s.NotEqual(id, id2)
}

func (s *CryptoTestSuite) TestGenerateVerificationCode() {
	for range 100 {
		code := GenerateVerificationCode()
		s.Len(code, 6, "验证码应为 6 位")
		for _, c := range code {
			s.True(c >= '0' && c <= '9', "验证码应只包含数字")
		}
	}
}

func (s *CryptoTestSuite) TestGenerateRandomInt32() {
	tests := []struct {
		name    string
		min     int32
		max     int32
		wantErr bool
	}{
		{name: "正常范围", min: 1, max: 10, wantErr: false},
		{name: "相邻值", min: 5, max: 6, wantErr: false},
		{name: "min 等于 max", min: 5, max: 5, wantErr: true},
		{name: "min 大于 max", min: 10, max: 1, wantErr: true},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			val, err := GenerateRandomInt32(tt.min, tt.max)
			if tt.wantErr {
				s.Error(err)
				return
			}
			s.NoError(err)
			s.GreaterOrEqual(val, tt.min)
			s.LessOrEqual(val, tt.max)
		})
	}
}

func (s *CryptoTestSuite) TestGenerateRandomInt64() {
	val, err := GenerateRandomInt64(100, 200)
	s.Require().NoError(err)
	s.GreaterOrEqual(val, int64(100))
	s.LessOrEqual(val, int64(200))

	_, err = GenerateRandomInt64(200, 100)
	s.Error(err)
}

func (s *CryptoTestSuite) TestGenerateBusinessID() {
	for range 100 {
		id, err := GenerateBusinessID()
		s.Require().NoError(err)
		s.GreaterOrEqual(id, int32(100000000))
		s.LessOrEqual(id, int32(999999999))
	}
}

func (s *CryptoTestSuite) TestGenerateBusinessID64() {
	for range 100 {
		id, err := GenerateBusinessID64()
		s.Require().NoError(err)
		s.GreaterOrEqual(id, int64(100000000000000000))
		s.LessOrEqual(id, int64(999999999999999999))
	}
}

func (s *CryptoTestSuite) TestHashAndVerifyPassword() {
	password := "SecureP@ss123"

	hashed, err := HashPassword(password)
	s.Require().NoError(err)
	s.NotEmpty(hashed)
	s.NotEqual(password, hashed)

	// 正确密码验证通过
	s.NoError(VerifyPassword(hashed, password))

	// 错误密码验证失败
	s.Error(VerifyPassword(hashed, "WrongPassword"))
}

func (s *CryptoTestSuite) TestHashPassword_DifferentHashes() {
	password := "SamePassword"

	h1, err := HashPassword(password)
	s.Require().NoError(err)

	h2, err := HashPassword(password)
	s.Require().NoError(err)

	// bcrypt 每次生成的哈希值不同含随机盐
	s.NotEqual(h1, h2)
}
