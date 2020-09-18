package main

import (
	"github.com/go-pg/pg/v10"
	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/certificate"
	"github.com/sideshow/apns2/payload"
	"log"
)

func SendNotification(deviceToken string) {
	cert, err := certificate.FromP12File("./cert.p12", "")
	if err != nil {
		log.Fatal("Cert Error:", err)
	}

	notification := &apns2.Notification{}
	notification.PushType = apns2.PushTypeBackground
	notification.DeviceToken = deviceToken
	notification.Topic = "com.elsdoerfer.photobeam"
	notification.Payload = payload.NewPayload().ContentAvailable()

	// If you want to test push notifications for builds running directly from XCode (Development), use
	// For apps published to the app store or installed as an ad-hoc distribution use Production()
	//client := apns2.NewClient(cert).Production()
	client := apns2.NewClient(cert).Development()
	res, err := client.Push(notification)

	if err != nil {
		log.Fatal("Error:", err)
	}

	log.Printf("%v %v %v\n", res.StatusCode, res.ApnsID, res.Reason)
}

func SendNotificationToAccountId(db *pg.DB, accountId int) error {
	// We need to get the device key for the target user.
	peerAccount := new(Account)
	err := db.Model(peerAccount).
		Where("id = ?", accountId).
		Limit(1).
		Select()
	if err != nil {
		return err;
	}

	if peerAccount.ApnsToken != "" {
		SendNotification(peerAccount.ApnsToken)
	}

	return nil
}
