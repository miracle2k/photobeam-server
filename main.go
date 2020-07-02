package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-pg/pg/v10"
	"github.com/satori/go.uuid"
	"log"
	rand "math/rand"
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
	if (r.Body == nil) {
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

func RegisterHandler(w http.ResponseWriter, r *http.Request){
	db := Connect()
	defer db.Close()

	account := &Account{
		Key: uuid.NewV4().String(),
		ConnectCode: ConnectCode(),
	}
	err := db.Insert(account)
	if err != nil {
		panic(err)
	}

	accountResponse := &AccountResponse{
		AccountId: account.Id,
		ConnectCode: account.ConnectCode,
		AuthKey: account.Key,
	}
	if err := json.NewEncoder(w).Encode(accountResponse); err != nil {
		panic(err)
	}
}

func ConnectHandler(w http.ResponseWriter, r *http.Request){
	db := Connect()
	defer db.Close()

	account, err := ReadAuth(db, r)
	if err != nil {
		panic(fmt.Sprintf("auth not found %s", err))
	}

	var args ConnectArguments
	err = GetFromReq(w, r, &args)
	if err != nil {
		panic("invalid code")
	}

	otherAccount := new(Account)
	err = db.Model(otherAccount).
		Where("connect_code = ?", args.ConnectCode).
		Limit(1).
		Select()
	if err != nil {
		panic("invalid code")
	}

	// Initiator closes their connection immediately.
	toDelete := &Connection{
		LeftId: account.Id,
	}
	_, err = db.Model(toDelete).Where("leftId = ?leftId || rightId = >leftId").Delete()

	// Create a new pending connection
	connection := &Connection{
		LeftId: account.Id,
		RightId: otherAccount.Id,
		Status: "pending",
	}

	err = db.Insert(connection)
	if err != nil {
		panic(err)
	}
}

func query(w http.ResponseWriter, r *http.Request){
	// out: {state: 'pending', shouldFetch: false, peerShouldFetch: false }
}

func setPicture(w http.ResponseWriter, r *http.Request){
	// out: {peerShouldFetch: 'true'}
}

func getPicture(w http.ResponseWriter, r *http.Request){
	// out: the image binary
}


func clearPicture(w http.ResponseWriter, r *http.Request){
	// out: {shouldFetch: 'false'}
}


func handleRequests() {
	http.HandleFunc("/register", RegisterHandler)
	http.HandleFunc("/connect", ConnectHandler)
	http.HandleFunc("/set", setPicture)
	http.HandleFunc("/get", getPicture)
	http.HandleFunc("/clear", clearPicture)

	log.Fatal(http.ListenAndServe(":10000", nil))
}

func main() {
	db := Connect()
	CreateSchema(db)
	//handleRequests()
}