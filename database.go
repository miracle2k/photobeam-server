package main

import (
	"github.com/go-pg/pg/v10"
	"github.com/go-pg/pg/v10/orm"
	"github.com/go-pg/pg/v10/pgext"
	"time"
)

type Account struct {
	Id int
	Key string
	ConnectCode string
	TimeCreated string
}

type Connection struct {
	Id          int
	InitiatorId int
	InviteeId   int
	Status      string   // pending, confirmed
	TimeCreated string
}

func (c *Connection) GetPeerId(userId int) int {
	if c.InitiatorId == userId {
		return c.InviteeId;
	}
	return c.InitiatorId
}

type Payload struct {
	ConnectionId int `pg:",pk"`
	FromId int `pg:",pk"`
	TimeCreated time.Time
	TimeFetched pg.NullTime
	Fetched bool

	// We store the photo itself here, but only because it is basically a temporary storage. It will
	// be cleared out as soon as the image is fetched. Just make sure you do ExcludeColumn().
	Data []byte
}


// Do createdb & dropdb for a full reset
func CreateSchema(db *pg.DB) error {
	models := []interface{}{
		(*Account)(nil),
		(*Connection)(nil),
		(*Payload)(nil),
	}

	for _, model := range models {
		err := db.CreateTable(model, &orm.CreateTableOptions{
			//Temp: true, // temp table
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func Connect() *pg.DB {
	db := pg.Connect(&pg.Options{
		Addr:     ":5432",
		User:     "postgres",
		Password: "",
		Database: "photobeam",
	})
	db.AddQueryHook(pgext.DebugHook{})
	return db
}

