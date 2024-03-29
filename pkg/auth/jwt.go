package auth

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/sfs/pkg/env"

	"github.com/dgrijalva/jwt-go" // TODO: replace -- this has a security problem
)

// json web token
type Token struct {
	Jwt    string // token string
	Secret []byte // secret key
}

func NewT() *Token {
	s, err := getSecret()
	if err != nil {
		log.Fatalf("unable to retrieve token secret: %v", err)
	}
	return &Token{
		Secret: s,
	}
}

func getSecret() ([]byte, error) {
	envCfg := env.NewE()
	if s, err := envCfg.Get("JWT_SECRET"); err == nil {
		return []byte(s), nil
	} else {
		return nil, err
	}
}

// retrieve jwt token from request
func (t *Token) Extract(rawReqToken string) (string, error) {
	splitToken := strings.Split(rawReqToken, " ")
	if len(splitToken) != 2 { // bearer token not in proper format
		return "", fmt.Errorf("invalid token format")
	}
	reqToken := strings.TrimSpace(splitToken[1])
	return reqToken, nil
}

// verify jwt token and attempt to retrieve the request payload.
//
// use the return value to compare against the db and whether
// they're an actual user
func (t *Token) Verify(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return t.Secret, nil
	})
	if err != nil {
		return "", err
	}
	if !token.Valid {
		return "", fmt.Errorf("invalid token")
	}
	// retrieve claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("failed to parse jwt claims")
	}
	// retrieve the payload as a string
	data := claims["sub"].(string)
	if data == "" {
		return "", fmt.Errorf("no payload found in token claims")
	}
	return data, nil
}

// validate a request token from a given http request
func (t *Token) Validate(r *http.Request) (string, error) {
	var rawToken = r.Header.Get("Authorization")
	if rawToken == "" {
		return "", fmt.Errorf("no token provided")
	}
	token, err := t.Extract(rawToken)
	if err != nil {
		return "", fmt.Errorf("failed to extract token: %v", err)
	}
	itemInfo, err := t.Verify(token)
	if err != nil {
		return "", err
	}
	return itemInfo, nil
}

// create a new token using a given payload/string
func (t *Token) Create(payload string) (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)
	claims["sub"] = payload
	// TODO: token expires in 1 hour by default.
	// expiration times should vary depending on the request.
	claims["exp"] = time.Now().Add(time.Hour).UTC()
	tokenString, err := token.SignedString(t.Secret)
	if err != nil {
		return "", err
	}
	t.Jwt = tokenString
	return tokenString, nil
}
