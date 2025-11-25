package main

import (
	"fmt"
	"log"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	rabbitURL = "amqp://guest:guest@localhost:5672/"
)

func main() {

	fmt.Println("Starting Peril server...")

	client, err := amqp.Dial(rabbitURL)
	if err != nil {
		log.Fatalf("Could not connect to rabbit")
	}

	defer client.Close()

	fmt.Println("The connection has been succesful")

	ch, err := client.Channel()
	if err != nil {
		log.Fatalf("Could not create channel")
	}

	// tempGameLog := routing.GameLog{
	// 	CurrentTime: time.Now(),
	// 	Message:     fmt.Sprintf("won a war against "),
	// 	Username:    "Me",
	// }

	// payload, err := routing.Encode(tempGameLog)

	// if err != nil {
	// 	fmt.Println("Error while encoding ", err.Error())
	// }

	// fmt.Println(payload)

	// gob, err := routing.Decode(payload)

	// if err != nil {
	// 	fmt.Println("Error while decoding ", err.Error())
	// }
	// fmt.Println(gob)

	// return

	pubsub.SubscribeGOB(
		client,
		routing.ExchangePerilTopic,
		"game_logs",
		"game_logs.*",
		pubsub.Durable,
		handleLog,
	)

	gamelogic.PrintServerHelp()

	for {
		inputSlice := gamelogic.GetInput()

		if len(inputSlice) >= 1 {
			if inputSlice[0] == "pause" {
				fmt.Println("Sending pause to rabbit q")
				pubsub.PublishJSON(ch, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{IsPaused: true})
			} else if inputSlice[0] == "resume" {
				fmt.Println("Sending resume to rabbit q")
				pubsub.PublishJSON(ch, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{IsPaused: false})
			} else if inputSlice[0] == "quit" {
				fmt.Println("Quiting the game")
				break
			} else {

				fmt.Println("Don't understand")
			}
		}
	}

}
func handleLog(gl routing.GameLog) pubsub.AckType {

	defer fmt.Println(">")

	err := gamelogic.WriteLog(gl)

	if err != nil {
		return pubsub.NackReque
	}

	return pubsub.RegularAck

}
