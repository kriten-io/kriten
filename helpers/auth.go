package helpers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log"
	"time"

	"github.com/kriten-io/kriten/config"
	"github.com/kriten-io/kriten/models"

	"github.com/golang-jwt/jwt"
	uuid "github.com/satori/go.uuid"
)

func CreateJWTToken(credentials *models.Credentials, userID uuid.UUID, jwtConf config.JWTConfig) (string, error) {
	expirationTime := time.Now().Add(time.Second * time.Duration(jwtConf.ExpirySeconds))

	claims := &models.Claims{
		Username: credentials.Username,
		UserID:   userID,
		Provider: credentials.Provider,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtConf.Key)
	if err != nil {
		log.Println(err)
		return "", err
	}
	return tokenString, nil
}

func ValidateJWTToken(tokenStr string, jwtConf config.JWTConfig) (*models.Claims, error) {
	claims := &models.Claims{}

	token, err := jwt.ParseWithClaims(tokenStr, claims,
		func(t *jwt.Token) (interface{}, error) {
			return jwtConf.Key, nil
		})

	if err != nil || !token.Valid {
		log.Println(err)
		return nil, errors.New("error: invalid token")
	}
	return claims, nil
}

func GenerateHMAC(apiSecret string, key string) string {
	// Create a new HMAC by defining the hash type and the key (as byte array)
	h := hmac.New(sha256.New, []byte(apiSecret))

	// Write Data to it
	h.Write([]byte(key))

	// Get result and encode as hexadecimal string
	sha := hex.EncodeToString(h.Sum(nil))

	return sha
}
