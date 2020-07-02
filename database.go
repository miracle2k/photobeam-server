package main

import (
	"github.com/go-pg/pg/v10"
	"github.com/go-pg/pg/v10/orm"
	"github.com/go-pg/pg/v10/pgext"
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

type Payload struct {
	ConnectionId int `pg:",pk"`
	FromId int `pg:",pk"`
	TimeCreated string
	TimeFetched string
	Fetched bool
}


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

