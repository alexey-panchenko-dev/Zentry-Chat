package main

import (
	"fmt"
	webpush "github.com/SherClockHolmes/webpush-go"
	"log"
)

func main() {
	private, public, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("VAPID_PUBLIC_KEY=" + public)
	fmt.Println("VAPID_PRIVATE_KEY=" + private)
}
