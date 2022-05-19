package repoAuthentication

import (
	"crypto/md5"
	"encoding/hex"

	"github.com/dgrijalva/jwt-go"
)

type CookieClaims struct {
	Login   string `json:"login"`
	Role    int    `json:"role"`
	Surname string `json:"surname"`
	Name    string `json:"name"`
	jwt.StandardClaims
}

type TarantoolAuthTuple struct {
	Login    string
	Password string
	UserId   string
}

type TarantoolVerifyTuple struct {
	Hash string
	Uuid string
}

func GetMD5(str string) (string, error) {
	hash := md5.New()
	_, err := hash.Write([]byte(str))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
