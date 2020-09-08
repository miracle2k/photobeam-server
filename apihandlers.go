package main

import (
	"bytes"
	"encoding/json"
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

/**
 * Called to connect to a peer. If you are already connected to someone, will unconnect.
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
	}
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

	connection := new(Connection)
	err := db.Model(connection).Where("invitee_id = ?0 OR initiator_id = ?1", account.Id, account.Id).Select()
	if err != nil {
		// TODO: What about connection errors?
		if !isBadConn(err, false) {
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

	accountShouldFetch, peerShouldFetch, err := QueryPayload(db, connection.Id, account.Id)
	if err != nil {
		log.Printf("QueryPayload failed: %s", err)
		http.Error(w, "could not query payload", http.StatusBadRequest)
		return
	}

	stateResponse := &StateResponse{
		PeerId: peerId,
		Status: status,
		ShouldFetch: accountShouldFetch,
		ShouldPeerFetch: peerShouldFetch,
	}
	if err := json.NewEncoder(w).Encode(stateResponse); err != nil {
		panic(err)
	}
}

/**
 * Accept a connection request from a peer (and break any existing connection).
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
		PeerId: 1,
		Status: "pending",
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

	// out: {peerShouldFetch: 'true'}
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

	// out: {peerShouldFetch: 'true'}
}
