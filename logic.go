package main

import (
	"fmt"
	"github.com/go-pg/pg/v10"
)

func LinkAccounts(db *pg.DB, initiator *Account, target *Account, Status string) error {
	// Initiator closes all their connections immediately.
	_, err := db.Model(new(Connection)).Where("left_id = ?0 OR right_id = ?0", initiator.Id).Delete()
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
		return err;
	}
	return nil
}

func AcceptLink(db *pg.DB, acceptor *Account, peerId int) error {
	// Find such a connection
	connection := new(Connection)
	err := db.Model(connection).Where("invitee_id = ?0 AND initiator_id = ?1", acceptor.Id, peerId).Select()
	if err != nil { return err; }

	// Set this connection to complete
	connection.Status = "live"

	// Update the connection
	err = db.Update(connection)
	if err != nil { return err; }

	// Delete all other connections by the acceptor.
	_, err = db.Model(new(Connection)).Where("(left_id = ?0 OR right_id = ?0) AND id != ?1", acceptor.Id, connection.Id).Delete()
	if err != nil {
		return err;
	}

	return nil;
}
