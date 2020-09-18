package main

import (
	"errors"
	"fmt"
	"github.com/go-pg/pg/v10"
	"github.com/go-pg/pg/v10/orm"
	"time"
)

/**
 * Find the current connection for this account.
 */
func GetConnection(db *pg.DB, accountId int) (*Connection, error) {
	// Find a connection for this user.
	connection := new(Connection)
	err := db.Model(connection).Where("invitee_id = ?0 OR initiator_id = ?0", accountId).Select()
	if err != nil {
		return nil, errors.New("User has no connection")
	}
	return connection, nil
}



func UnlinkAnyConnection(db *pg.DB, account *Account, connectionIdToKeep int) error {
	var query *orm.Query;
	if connectionIdToKeep != 0 {
		query = db.Model(new(Connection)).Where("(invitee_id = ?0 OR initiator_id = ?0) AND id != ?1", account.Id, connectionIdToKeep)
	} else {
		query = db.Model(new(Connection)).Where("invitee_id = ?0 OR initiator_id = ?0", account.Id)
	}
	_, err := query.Delete()
	if err != nil {
		panic(fmt.Sprintf("failed to delete %s", err))
	}

	// TODO: Delete all payloads

	return nil;
}


/**
 * Create a pending connection to the target, and leave any current connections.
 */
func LinkAccounts(db *pg.DB, initiator *Account, target *Account, Status string) error {
	// Initiator closes all their connections immediately.
	err := UnlinkAnyConnection(db, initiator, 0)
	if err != nil {
		panic(fmt.Sprintf("failed to delete %s", err))
	}

	// Create a new pending connection
	connection := &Connection{
		InitiatorId: initiator.Id,
		InviteeId:   target.Id,
		Status:      Status,
	}

	err = db.Insert(connection)
	if err != nil {
		return err
	}
	return nil
}

/**
 * Accept a pending connection request, break any existing connection.
 */
func AcceptLink(db *pg.DB, acceptor *Account, peerId int) error {
	// Find such a connection
	connection := new(Connection)
	err := db.Model(connection).Where("invitee_id = ?0 AND initiator_id = ?1", acceptor.Id, peerId).Select()
	if err != nil {
		return err
	}

	// Set this connection to complete
	connection.Status = "live"

	// Update the connection
	err = db.Update(connection)
	if err != nil {
		return err
	}

	// Delete all other connections by the acceptor.
	err = UnlinkAnyConnection(db, acceptor, connection.Id)
	if err != nil {
		return err
	}


	return nil
}

/**
 * A user sets a new payload for the partner.
 */
func RecordNewPayload(db *pg.DB, senderId int, data []byte) (int, error) {
	// Find a connection for this user.
	connection, err := GetConnection(db, senderId)
	if err != nil {
		return 0, err
	}

	// Create a new payload record
	payload := &Payload{
		ConnectionId: connection.Id,
		FromId:       senderId,
		TimeCreated:  time.Now(),
		Fetched:      false,
		Data:         data,
	}

	err = db.Insert(payload)
	if err != nil {
		return 0, err
	}

	return connection.GetPeerId(senderId), nil
}

/**
 * Check if there is a payload waiting for either user in this connection.
 * Return is: (accountHas, peerHas)
 */
func QueryPayload(db *pg.DB, connectionId int, accountId int) (bool, bool, error) {
	var payloads []Payload
	err := db.Model(&payloads).
		Where("connection_id = ? AND fetched IS NULL", connectionId).
		Limit(2).
		Select();

	if err != nil {
		return false, false, err
	}

	if len(payloads) == 0 {
		return false, false, nil
	} else if len(payloads) == 1 {
		if payloads[0].FromId == accountId {
			return false, true, nil
		}  else {
			return true, false, nil
		}
	} else {
		return true, true, nil
	}
}

/**
 * Get the payload for this user to download.
 */
func FetchPayload(db *pg.DB, fetcherId int) ([]byte, error) {
	// Find a connection for this user.
	connection, err := GetConnection(db, fetcherId)
	if err != nil {
		return nil, errors.New("User has no connection")
	}

	peerId := connection.GetPeerId(fetcherId)

	// Find a payload
	payload := new(Payload)
	err = db.Model(payload).Where("connection_id = ?0 AND from_id = ?1", connection.Id, peerId).Select()
	if err != nil {
		return nil, errors.New("No payload available")
	}

	if payload.Fetched {
		return nil, errors.New("Payload already fetched")
	}

	return payload.Data, nil
}

/**
 * Once a client has the payload safely in their hands, delete it.
 */
func ClearPayload(db *pg.DB, fetcherId int) error {
	// Find a connection for this user.
	connection := new(Connection)
	err := db.Model(connection).Where("invitee_id = ?0 OR initiator_id = ?0", fetcherId).Select()
	if err != nil {
		return errors.New("User has no connection")
	}

	peerId := connection.GetPeerId(fetcherId)

	// Find a payload
	payload := new(Payload)
	err = db.Model(payload).Where("connection_id = ?0 AND from_id = ?1", connection.Id, peerId).Select()
	if err != nil {
		return err
	}

	if payload.Fetched {
		return nil
	}

	payload.Fetched = true
	payload.Data = nil

	err = db.Update(payload)
	if err != nil {
		return err
	}

	return nil
}
