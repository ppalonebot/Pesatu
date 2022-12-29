package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt"
)

const hmacSecret = "WjdwZUh2dWJGdFB1UWRybg=="
// const defaulExpireTime = 604800 // 1 week

type ExpireTime int
const (
	AWeek ExpireTime = 604800
	ADay ExpireTime = 86400
	AnHour ExpireTime = 3600
)
type Claims struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	jwt.StandardClaims
}

func (c *Claims) GetUID() string {
	return c.ID
}

func (c *Claims) GetUsername() string {
	return c.Name
}

// CreateJWTToken generates a JWT signed token for for the given user id and username
func CreateJWTToken(id string, username string) (string, error) {
	return CreateJWTWithExpire(id,username,AWeek)
}

func CreateJWTWithExpire(id string, username string, expired ExpireTime) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"ID":        id,
		"Name":      username,
		"ExpiresAt": time.Now().Unix() + int64(expired),
	})
	tokenString, err := token.SignedString([]byte(hmacSecret))

	return tokenString, err
}

func ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Don't forget to validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		// hmacSampleSecret is a []byte containing your secret, e.g. []byte("my_secret_key")
		return []byte(hmacSecret), nil
	})

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	} else {
		return nil, err
	}
}
