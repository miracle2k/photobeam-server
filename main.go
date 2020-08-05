package main

import (
	. "bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-pg/pg/v10"
	"github.com/satori/go.uuid"
	"io"
	"log"
	rand "math/rand"
	"net/http"
	"os"
	"time"
	"github.com/urfave/cli/v2" // imports as package "cli"
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

/**
 * Called by apps to create a new account. They will get a secret key and a code for peers to connect.
 */
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

/**
 * Called to connect to a peer. If you are already connected to someone, will unconnect.
 */
func ConnectHandler(w http.ResponseWriter, r *http.Request){
	db := Connect()
	defer db.Close()

	account, err := ReadAuth(db, r)
	if err != nil {
		http.Error(w, fmt.Sprintf("auth not found %s", err), http.StatusUnauthorized)
		return
	}

	var args ConnectArguments
	err = GetFromReq(w, r, &args)
	if err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	otherAccount := new(Account)
	err = db.Model(otherAccount).
		Where("connect_code = ?", args.ConnectCode).
		Limit(1).
		Select()
	if err != nil {
		http.Error(w, "invalid code", http.StatusBadRequest)
		return
	}

	err = LinkAccounts(db, account, otherAccount, "pending")
	if err != nil {
		log.Printf("LinkAccounts failed: %s", err)
		http.Error(w, "could not link accounts", http.StatusBadRequest)
		return
	}

	stateResponse := &StateResponse{
		PeerId: otherAccount.Id,
		Status: "pending",
	}
	if err := json.NewEncoder(w).Encode(stateResponse); err != nil {
		panic(err)
	}
}


/**
 * Return the current state of your account, including connected to who? Are there pending requests?
 */
func QueryHandler(w http.ResponseWriter, r *http.Request){
	stateResponse := &StateResponse{
		PeerId: 1,
		Status: "pending",
	}
	if err := json.NewEncoder(w).Encode(stateResponse); err != nil {
		panic(err)
	}

	// out: {state: 'pending', shouldFetch: false, peerShouldFetch: false }
}


/**
 * Accept a connection request from a peer (and break any existing connection).
 */
func AcceptHandler(w http.ResponseWriter, r *http.Request){
	db := Connect()
	defer db.Close()

	actorAccount, err := ReadAuth(db, r)
	if err != nil {
		http.Error(w, fmt.Sprintf("auth not found %s", err), http.StatusUnauthorized)
		return
	}

	var args AcceptArguments
	err = GetFromReq(w, r, &args)
	if err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	// If there is a connection from this peer, accept it.
	err = AcceptLink(db, actorAccount, args.PeerId)
	if err != nil {
		http.Error(w, "failed to accept", http.StatusBadRequest)
		return
	}

	stateResponse := &StateResponse{
		PeerId: 1,
		Status: "pending",
	}
	if err := json.NewEncoder(w).Encode(stateResponse); err != nil {
		panic(err)
	}
}

/**
 * Set a payload for the current connection.
 */
func setPicture(w http.ResponseWriter, r *http.Request){
	db := Connect()
	defer db.Close()

	actorAccount, err := ReadAuth(db, r)
	if err != nil {
		http.Error(w, fmt.Sprintf("auth not found %s", err), http.StatusUnauthorized)
		return
	}

	err = r.ParseMultipartForm(1024 * 1024 * 5)
	if err != nil {
		http.Error(w, "failed to parse request", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "failed to find file", http.StatusBadRequest)
		return
	}
	defer file.Close()


	buf := NewBuffer(nil)
	if _, err := io.Copy(buf, file); err != nil {
		http.Error(w, "failed to copy file", http.StatusInternalServerError)
		return
	}

	err = RecordNewPayload(db, actorAccount.Id, buf.Bytes())
	if err != nil {
		http.Error(w, "failed to record payload", http.StatusBadRequest)
		return
	}

	// out: {peerShouldFetch: 'true'}
}

func getPicture(w http.ResponseWriter, r *http.Request){
	db := Connect()
	defer db.Close()

	actorAccount, err := ReadAuth(db, r)
	if err != nil {
		http.Error(w, fmt.Sprintf("auth not found %s", err), http.StatusUnauthorized)
		return
	}

	data, err := FetchPayload(db, actorAccount.Id)
	if err != nil {
		http.Error(w, "No payload available", http.StatusBadRequest)
		return
	}

	w.Write(data)
}


func clearPicture(w http.ResponseWriter, r *http.Request){
	db := Connect()
	defer db.Close()

	actorAccount, err := ReadAuth(db, r)
	if err != nil {
		http.Error(w, fmt.Sprintf("auth not found %s", err), http.StatusUnauthorized)
		return
	}

	err = ClearPayload(db, actorAccount.Id)
	if err != nil {
		http.Error(w, "No payload available", http.StatusBadRequest)
		return
	}

	// out: {peerShouldFetch: 'true'}
}


func handleRequests() {
	http.HandleFunc("/register", RegisterHandler)
	http.HandleFunc("/connect", ConnectHandler)
	http.HandleFunc("/query", QueryHandler)
	http.HandleFunc("/accept", AcceptHandler)
	http.HandleFunc("/set", setPicture)
	http.HandleFunc("/get", getPicture)
	http.HandleFunc("/clear", clearPicture)

	log.Fatal(http.ListenAndServe(":10000", nil))
}

func main() {
	app := &cli.App{
		Name: "photobeam-server",
		Usage: "go beam!",
		Commands: []*cli.Command{
			{
				Name:    "run",
				Usage:   "run the server",
				Action:  func(c *cli.Context) error {
					handleRequests()
					return nil
				},
			},
			{
				Name:    "createdb",
				Usage:   "create the database",
				Action:  func(c *cli.Context) error {
					db := Connect()
					CreateSchema(db)
					return nil
				},
			},
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}