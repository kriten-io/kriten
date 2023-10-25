package helpers

import (
	"errors"
	"kriten/config"
	"kriten/models"
	"log"
	"time"

	"github.com/golang-jwt/jwt"
	uuid "github.com/satori/go.uuid"
)

func CreateToken(credentials *models.Credentials, userID uuid.UUID, jwtConf config.JWTConfig) (string, error) {
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

func ValidateToken(tokenStr string, jwtConf config.JWTConfig) (*models.Claims, error) {
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
