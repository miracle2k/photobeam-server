package main

import (
	"github.com/urfave/cli/v2" // imports as package "cli"
	"log"
	"net/http"
	"os"
)

func handleRequests() {
	http.HandleFunc("/register", RegisterHandler)
	http.HandleFunc("/connect", ConnectHandler)
	http.HandleFunc("/disconnect", DisconnectHandler)
	http.HandleFunc("/query", QueryHandler)
	http.HandleFunc("/accept", AcceptHandler)
	http.HandleFunc("/set", SetPictureHandler)
	http.HandleFunc("/get", GetPictureHandler)
	http.HandleFunc("/clear", ClearPictureHandler)

	log.Print("Running on port :10000")
	log.Fatal(http.ListenAndServe(":10000", nil))
}

func main() {
	app := &cli.App{
		Name:  "photobeam-server",
		Usage: "go beam!",
		Commands: []*cli.Command{
			{
				Name:  "run",
				Usage: "run the server",
				Action: func(c *cli.Context) error {
					handleRequests()
					return nil
				},
			},
			{
				Name:  "createdb",
				Usage: "create the database",
				Action: func(c *cli.Context) error {
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
