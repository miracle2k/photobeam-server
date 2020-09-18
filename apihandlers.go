package main

import (
	"bytes"
	"encoding/json"
	"github.com/go-pg/pg/v10"
	"github.com/satori/go.uuid"
	"io"
	"log"
	"net/http"
)

/**
 * Called by apps to create a new account. They will get a secret key and a code for peers to connect.
 */
func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	db := Connect()
	defer db.Close()

	account := &Account{
		Key:         uuid.NewV4().String(),
		ConnectCode: ConnectCode(),
	}
	err := db.Insert(account)
	if err != nil {
		panic(err)
	}

	accountResponse := &AccountResponse{
		AccountId:   account.Id,
		ConnectCode: account.ConnectCode,
		AuthKey:     account.Key,
	}
	if err := json.NewEncoder(w).Encode(accountResponse); err != nil {
		panic(err)
	}
}

func SetPropsHandler(w http.ResponseWriter, r *http.Request) {
	db := Connect()
	defer db.Close()

	canAccess, account := ValidateAuth(db, r, w)
	if !canAccess {
		return
	}

	var args SetPropsArguments
	err := GetFromReq(w, r, &args)
	if err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if args.ApnsToken != nil {
		account.ApnsToken = *args.ApnsToken
	}

	// Update the account
	err = db.Update(account)
	if err != nil {
		http.Error(w, "error changing props", http.StatusInternalServerError)
		return
	}

	accountResponse := &AccountResponse{
		AccountId:   account.Id,
		ConnectCode: account.ConnectCode,
		// We probably do not want to return the key again
		AuthKey:     account.Key,
	}
	if err := json.NewEncoder(w).Encode(accountResponse); err != nil {
		panic(err)
	}
}

/**
 * Called to connect to a peer. If you are already connected to someone, will unconnect.
 *
 * Argument includes a connection code. If there is no peer with this code, return status 400.
 *
 * Otherwise, return a State update.
 */
func ConnectHandler(w http.ResponseWriter, r *http.Request) {
	db := Connect()
	defer db.Close()

	canAccess, account := ValidateAuth(db, r, w)
	if !canAccess {
		return
	}

	var args ConnectArguments
	err := GetFromReq(w, r, &args)
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

	err = LinkAccounts(db, account, otherAccount, PENDING)
	if err != nil {
		log.Printf("LinkAccounts failed: %s", err)
		http.Error(w, "could not link accounts", http.StatusBadRequest)
		return
	}

	stateResponse := &StateResponse{
		PeerId: otherAccount.Id,
		Status: "pending",
		ShouldFetch: false,
		ShouldPeerFetch: false,
	}
	if err := json.NewEncoder(w).Encode(stateResponse); err != nil {
		panic(err)
	}
}


/**
 * Called to disconnect from the current connection.
 */
func DisconnectHandler(w http.ResponseWriter, r *http.Request) {
	db := Connect()
	defer db.Close()

	canAccess, account := ValidateAuth(db, r, w)
	if !canAccess {
		return
	}

	err := UnlinkAnyConnection(db, account, 0)
	if err != nil {
		log.Printf("UnlinkAnyConnection failed: %s", err)
		http.Error(w, "could not unlink connection", http.StatusBadRequest)
		return
	}

	stateResponse := &StateResponse{}
	if err := json.NewEncoder(w).Encode(stateResponse); err != nil {
		panic(err)
	}
}


/**
 * Return the current state of your account, including connected to who? Are there pending requests?
 */
func QueryHandler(w http.ResponseWriter, r *http.Request) {
	db := Connect()
	defer db.Close()

	canAccess, account := ValidateAuth(db, r, w)
	if !canAccess {
		return
	}

	connection, err := GetConnection(db, account.Id)
	if err != nil {
		if isBadConn(err, false) {
			panic(err);
			return;
		}
		stateResponse := &StateResponse{
			PeerId: 0,
			Status: "",
			ShouldFetch: false,
			ShouldPeerFetch: false,
		}
		if err := json.NewEncoder(w).Encode(stateResponse); err != nil {
			panic(err)
		}
		return;
	}

	peerId := connection.GetPeerId(account.Id)
	status := ""
	if connection.Status == PENDING {
		if connection.InviteeId == account.Id {
			status = "pendingWithMe"
		} else {
			status = "pendingWithPeer"
		}
	} else {
		status = "connected"
	}

	stateResponse := &StateResponse{
		PeerId: peerId,
		Status: status,
	}
	err = CompleteFetchResponse(stateResponse, db, connection, account)
	if err != nil {
		log.Printf("QueryPayload failed: %s", err)
		http.Error(w, "could not query payload", http.StatusBadRequest)
		return
	}

	if err := json.NewEncoder(w).Encode(stateResponse); err != nil {
		panic(err)
	}
}

func CompleteFetchResponse(response *StateResponse, db *pg.DB, connection *Connection, account *Account) error {
	accountShouldFetch, peerShouldFetch, err := QueryPayload(db, connection.Id, account.Id)
	if err != nil {
		return err;
	}

	response.ShouldPeerFetch = peerShouldFetch;
	response.ShouldFetch = accountShouldFetch;
	return nil
}

func WriteBackConnectedResponse(w http.ResponseWriter, db *pg.DB, account *Account) {
	connection, err := GetConnection(db, account.Id)
	if err != nil {
		http.Error(w, "unexpectedly connection is missing", http.StatusBadRequest)
		return
	}

	stateResponse := &StateResponse{
		PeerId: connection.GetPeerId(account.Id),
		Status: "connected",
	}
	err = CompleteFetchResponse(stateResponse, db, connection, account)
	if err != nil {
		log.Printf("QueryPayload failed: %s", err)
		http.Error(w, "could not query payload", http.StatusBadRequest)
		return
	}
	if err := json.NewEncoder(w).Encode(stateResponse); err != nil {
		panic(err)
	}
}

/**
 * Accept a connection request from a peer (and break any existing connection).
 * TODO: Support "no"
 */
func AcceptHandler(w http.ResponseWriter, r *http.Request) {
	db := Connect()
	defer db.Close()

	canAccess, actorAccount := ValidateAuth(db, r, w)
	if !canAccess {
		return
	}

	var args AcceptArguments
	err := GetFromReq(w, r, &args)
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
		PeerId: args.PeerId,
		Status: "connected",
		ShouldFetch: false,
		ShouldPeerFetch: false,
	}
	if err := json.NewEncoder(w).Encode(stateResponse); err != nil {
		panic(err)
	}
}

/**
 * Set a payload for the current connection. This is a multipart form request with the following keys:

 * file: the file to be uploaded
 */
func SetPictureHandler(w http.ResponseWriter, r *http.Request) {
	db := Connect()
	defer db.Close()

	canAccess, actorAccount := ValidateAuth(db, r, w)
	if !canAccess {
		return
	}

	err := r.ParseMultipartForm(1024 * 1024 * 5)
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

	buf := bytes.NewBuffer(nil)
	if _, err := io.Copy(buf, file); err != nil {
		http.Error(w, "failed to copy file", http.StatusInternalServerError)
		return
	}

	err = RecordNewPayload(db, actorAccount.Id, buf.Bytes())
	if err != nil {
		http.Error(w, "failed to record payload", http.StatusBadRequest)
		return
	}

	WriteBackConnectedResponse(w, db, actorAccount)
}

func GetPictureHandler(w http.ResponseWriter, r *http.Request) {
	db := Connect()
	defer db.Close()

	canAccess, actorAccount := ValidateAuth(db, r, w)
	if !canAccess {
		return
	}

	data, err := FetchPayload(db, actorAccount.Id)
	if err != nil {
		http.Error(w, "No payload available", http.StatusBadRequest)
		return
	}

	w.Write(data)
}

func ClearPictureHandler(w http.ResponseWriter, r *http.Request) {
	db := Connect()
	defer db.Close()

	canAccess, actorAccount := ValidateAuth(db, r, w)
	if !canAccess {
		return
	}

	err := ClearPayload(db, actorAccount.Id)
	if err != nil {
		http.Error(w, "No payload available", http.StatusBadRequest)
		return
	}

	WriteBackConnectedResponse(w, db, actorAccount)
}
