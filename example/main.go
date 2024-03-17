package main

import (
	"context"
	"os"

	"github.com/muhamadazmy/restate-sdk-go/server"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	UserSessionServiceName = "UserSession"
	TicketServiceName      = "TicketService"
	CheckoutServiceName    = "Checkout"
)

func main() {

	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	server := server.NewRestate().
		Bind(UserSessionServiceName, UserSession).
		Bind(TicketServiceName, TicketService).
		Bind(CheckoutServiceName, Checkout)

	if err := server.Start(context.Background(), ":9080"); err != nil {
		log.Error().Err(err).Msg("application exited unexpectedly")
		os.Exit(1)
	}
}
