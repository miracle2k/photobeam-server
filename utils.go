package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-pg/pg/v10"
	"math/rand"
	"net"
	"net/http"
	"time"
)

var seededRand *rand.Rand = rand.New(
	rand.NewSource(time.Now().UnixNano()))

func StringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func ConnectCode() string {
	// (26 * 2 + 10) ^ 4 = 14,776,336
	return StringWithCharset(4, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
}

func GetFromReq(w http.ResponseWriter, r *http.Request, item interface{}) error {
	if r.Body == nil {
		return errors.New("No body")
	}
	return json.NewDecoder(http.MaxBytesReader(w, r.Body, 1048576)).Decode(item)
}

func ReadAuth(db *pg.DB, r *http.Request) (*Account, error) {
	authKey := r.Header.Get("Authorization")
	account := new(Account)
	err := db.Model(account).
		Where("key = ?", authKey).
		Limit(1).
		Select()
	fmt.Print(account)
	return account, err
}

func ValidateAuth(db *pg.DB, r *http.Request, w http.ResponseWriter) (bool, *Account) {
	actorAccount, err := ReadAuth(db, r)
	if err != nil {
		if isBadConn(err, false) {
			panic(err);
		}
		http.Error(w, fmt.Sprintf("auth not found: %s", err), http.StatusUnauthorized)
		return false, nil
	}
	return true, actorAccount
}

func isBadConn(err error, allowTimeout bool) bool {
	if err == nil {
		return false
	}
	fmt.Print(err)
	if pgErr, ok := err.(pg.Error); ok {
		return pgErr.Field('S') == "FATAL"
	}
	if allowTimeout {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return !netErr.Temporary()
		}
	}
	return true
}