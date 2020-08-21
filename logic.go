package main

import (
	"errors"
	"fmt"
	"github.com/go-pg/pg/v10"
	"time"
)

/**
 * Create a pending connection to the target, and leave any current connections.
 */
func LinkAccounts(db *pg.DB, initiator *Account, target *Account, Status string) error {
	// Initiator closes all their connections immediately.
	_, err := db.Model(new(Connection)).Where("invitee_id = ?0 OR initiator_id = ?0", initiator.Id).Delete()
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
	_, err = db.Model(new(Connection)).Where("(left_id = ?0 OR right_id = ?0) AND id != ?1", acceptor.Id, connection.Id).Delete()
	if err != nil {
		return err
	}
	// TODO: Delete all payloads

	return nil
}

/**
 * A user set a new payload for the partner.
 */
func RecordNewPayload(db *pg.DB, senderId int, data []byte) error {
	// Find a connection for this user.
	connection := new(Connection)
	err := db.Model(connection).Where("invitee_id = ?0 OR initiator_id = ?0", senderId).Select()
	if err != nil {
		return err
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
		return err
	}
	return nil
}

/**
 * Check if there is a payload waiting for either user in this connection.
 * Return is: (accountHas, peerHas)
 */
func QueryPayload(db *pg.DB, connectionId int, accountId int) (bool, bool, error) {
	var payloads []Payload
	err := db.Model(&payloads).
		Where("connectionId = ? AND fetched = false", connectionId).
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
 * A user set a new payload for the partner.
 */
func FetchPayload(db *pg.DB, fetcherId int) ([]byte, error) {
	// Find a connection for this user.
	connection := new(Connection)
	err := db.Model(connection).Where("invitee_id = ?0 OR initiator_id = ?0", fetcherId).Select()
	if err != nil {
		return nil, errors.New("User has no connection")
	}

	peerId := connection.GetPeerId(fetcherId)

	// Find a payload
	payload := new(Payload)
	err = db.Model(payload).Where("connection_id = ?0 OR from_id = ?1", connection.Id, peerId).Select()
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
	err = db.Model(payload).Where("connection_id = ?0 OR from_id = ?1", connection.Id, peerId).Select()
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
