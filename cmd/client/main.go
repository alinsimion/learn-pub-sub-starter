package main

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	rabbitURL = "amqp://guest:guest@localhost:5672/"
)

func main() {
	fmt.Println("Starting Peril client...")

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
	defer ch.Close()

	name, err := gamelogic.ClientWelcome()
	if err != nil {
		log.Fatal("Could not read name from user")
	}

	userQueueName := fmt.Sprintf("%s.%s", routing.PauseKey, name)
	userArmyMovesQueueName := fmt.Sprintf("army_moves.%s", name)
	userWarKey := fmt.Sprintf("%s.%s", routing.WarRecognitionsPrefix, name)

	pubsub.DeclareAndBind(client, routing.ExchangePerilDirect, userQueueName, routing.PauseKey, pubsub.Transient)

	gamestate := gamelogic.NewGameState(name)

	pubsub.SubscribeJSON(
		client,
		routing.ExchangePerilDirect,
		userQueueName,
		routing.PauseKey,
		pubsub.Transient,
		handlerPause(gamestate),
	)

	pubsub.SubscribeJSON(
		client,
		routing.ExchangePerilTopic,
		userArmyMovesQueueName,
		"army_moves.*",
		pubsub.Transient,
		handlerMove(gamestate, ch),
	)

	pubsub.SubscribeJSON(
		client,
		routing.ExchangePerilTopic,
		"war",
		userWarKey,
		pubsub.Durable,
		handlerWar(gamestate, ch),
	)

	for {
		inputSlice := gamelogic.GetInput()

		if len(inputSlice) >= 1 {
			if inputSlice[0] == "spawn" {
				fmt.Printf("Got the spawn command with args: %s\n", inputSlice)
				err := gamestate.CommandSpawn(inputSlice)
				if err != nil {
					fmt.Printf("Error while spawning %s\n", err.Error())
				}

			} else if inputSlice[0] == "move" {
				fmt.Printf("Got the move command with args: %s\n", inputSlice)
				armyMove, err := gamestate.CommandMove(inputSlice)

				if err != nil {
					fmt.Printf("Error while moving %s\n", err.Error())
				} else {
					fmt.Println("Move succesful")
					pubsub.PublishJSON(ch, routing.ExchangePerilTopic, userArmyMovesQueueName, armyMove)
					fmt.Println("Move succesfully published")
				}
			} else if inputSlice[0] == "status" {
				fmt.Printf("Got the statu command with args: %s\n", inputSlice)
				gamestate.CommandStatus()

			} else if inputSlice[0] == "help" {
				gamelogic.PrintClientHelp()

			} else if inputSlice[0] == "spam" {
				counter, _ := strconv.Atoi(inputSlice[1])

				for counter > 0 {
					logMessage := gamelogic.GetMaliciousLog()

					tempGameLog := routing.GameLog{
						Message:     logMessage,
						Username:    name,
						CurrentTime: time.Now(),
					}

					pubsub.PublishGOB(ch, routing.ExchangePerilTopic, fmt.Sprintf("game_logs.%s", name), tempGameLog)

					counter -= 1
				}

			} else if inputSlice[0] == "quit" {
				gamelogic.PrintQuit()
				break

			} else {

				fmt.Println("Don't understand")
			}
		}
	}

}

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) pubsub.AckType {
	return func(ps routing.PlayingState) pubsub.AckType {
		gs.HandlePause(ps)
		defer fmt.Print(">")
		return pubsub.RegularAck
	}
}

func handlerMove(gs *gamelogic.GameState, ch *amqp.Channel) func(move gamelogic.ArmyMove) pubsub.AckType {
	return func(move gamelogic.ArmyMove) pubsub.AckType {
		outcome := gs.HandleMove(move)
		defer fmt.Print(">")
		switch outcome {
		case gamelogic.MoveOutComeSafe:
			return pubsub.RegularAck
		case gamelogic.MoveOutcomeMakeWar:
			userWarKey := fmt.Sprintf("%s.%s", routing.WarRecognitionsPrefix, move.Player.Username)

			rec := gamelogic.RecognitionOfWar{
				Attacker: move.Player,
				Defender: gs.GetPlayerSnap(),
			}
			err := pubsub.PublishJSON(ch, routing.ExchangePerilTopic, userWarKey, rec)

			if err != nil {
				fmt.Println("Error while publisihing the war move")
				return pubsub.NackReque
			}

			return pubsub.RegularAck
		case gamelogic.MoveOutcomeSamePlayer:
			return pubsub.NackDiscard
		default:
			return pubsub.NackDiscard
		}
	}
}

func handlerWar(gs *gamelogic.GameState, ch *amqp.Channel) func(rw gamelogic.RecognitionOfWar) pubsub.AckType {
	return func(rw gamelogic.RecognitionOfWar) pubsub.AckType {
		defer fmt.Print(">")
		outcome, winner, loser := gs.HandleWar(rw)
		switch outcome {
		case gamelogic.WarOutcomeNotInvolved:
			return pubsub.NackReque
		case gamelogic.WarOutcomeNoUnits:
			return pubsub.NackDiscard
		case gamelogic.WarOutcomeOpponentWon:
			tempGameLog := routing.GameLog{
				CurrentTime: time.Now(),
				Message:     fmt.Sprintf("%s won a war against %s", winner, loser),
				Username:    rw.Attacker.Username,
			}

			return publishGameLog(tempGameLog, ch)
		case gamelogic.WarOutcomeYouWon:

			tempGameLog := routing.GameLog{
				CurrentTime: time.Now(),
				Message:     fmt.Sprintf("%s won a war against %s", winner, loser),
				Username:    rw.Attacker.Username,
			}

			return publishGameLog(tempGameLog, ch)
		case gamelogic.WarOutcomeDraw:

			tempGameLog := routing.GameLog{
				CurrentTime: time.Now(),
				Message:     fmt.Sprintf("A war between %s and %s resulted in a draw", winner, loser),
				Username:    rw.Attacker.Username,
			}

			return publishGameLog(tempGameLog, ch)
		default:
			return pubsub.NackDiscard
		}
	}
}

func publishGameLog(gl routing.GameLog, ch *amqp.Channel) pubsub.AckType {

	userLogsKey := fmt.Sprintf("%s.%s", routing.GameLogSlug, gl.Username)

	err := pubsub.PublishGOB(ch, string(routing.ExchangePerilTopic), userLogsKey, gl)

	if err != nil {
		fmt.Println("Error while publishing gamelog ", err.Error())
		return pubsub.NackReque
	}

	return pubsub.RegularAck
}
