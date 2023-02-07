package auth

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
)

var hmacSecret string = "WjdwZUh2dWJGdFB1UWRybg=="

func SetHmacSecret(s string) {
	hmacSecret = s
}

type Claims struct {
	jwt.MapClaims
	ID        string `json:"id"`
	Usr       string `json:"usr"`
	Cmd       string `json:"cmd"`
	Code      string `json:"code"`
	ExpiresAt int64  `json:"exp"`
}

func (c *Claims) GetUID() string {
	return c.ID
}

func (c *Claims) GetUsername() string {
	return c.Usr
}

func (c *Claims) GetCode() string {
	return c.Code
}

func (c *Claims) GetCmd() string {
	return c.Cmd
}

func (c *Claims) IsExpired() bool {
	expiresAt := time.Unix(c.ExpiresAt, 0)
	if time.Now().After(expiresAt) {
		return true //claims, fmt.Errorf("token expired")
	}

	return false
}

// CreateJWTToken generates a JWT signed token for for the given user id and username
func CreateJWTToken(id, username, code string) (string, error) {
	return CreateJWTWithExpire(id, username, "Login", code, time.Hour*24)
}

func CreateJWTWithExpire(id string, username string, cmd string, code string, expired time.Duration) (string, error) {
	claims := Claims{
		MapClaims: jwt.MapClaims{},
		ID:        id,
		Usr:       username,
		Cmd:       cmd,
		Code:      code,
	}

	claims.ExpiresAt = time.Now().Add(expired).Unix()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
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

	if token == nil {
		return nil, err
	}

	// Get the custom claims from the token
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, err
	}

	//Check the expiration time
	// expiresAt := time.Unix(claims.ExpiresAt, 0)
	// if time.Now().After(expiresAt) {
	// 	return nil, fmt.Errorf("token expired")
	// }

	return claims, nil
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var token string
		parts := strings.Split(c.GetHeader("Authorization"), " ")
		if len(parts) == 2 {
			token = parts[1]
		}

		if len(token) == 0 {
			token, _ = c.Cookie("jwt")
		}

		socketFlag := false
		if len(token) == 0 {
			token = c.Query("jwt")
			socketFlag = len(token) > 0
		}

		if len(token) == 0 {
			c.Next()
			return
		}

		validuser, _ := ValidateToken(token)
		if validuser == nil {
			c.Next()
			return
		}

		if socketFlag && validuser.Cmd != "socket" {
			c.Next()
			return
		}

		c.Set("validuser", validuser)
		c.Next()
	}
}
