package main

import (
	"errors"

	"github.com/muhamadazmy/restate-sdk-go"
)

type TicketStatus int

const (
	TicketAvailable TicketStatus = 0
	TicketReserved  TicketStatus = 1
	TicketSold      TicketStatus = 2
)

func reserve(ctx restate.Context, _ string, _ restate.Void) (bool, error) {
	status, err := restate.GetAs[TicketStatus](ctx, "status")
	if err != nil && !errors.Is(err, restate.ErrKeyNotFound) {
		return false, err
	}

	if status == TicketAvailable {
		return true, restate.SetAs(ctx, "status", TicketReserved)
	}

	return false, nil
}

func unreserve(ctx restate.Context, _ string, _ restate.Void) (void restate.Void, err error) {
	status, err := restate.GetAs[TicketStatus](ctx, "status")
	if err != nil && !errors.Is(err, restate.ErrKeyNotFound) {
		return void, err
	}

	if status != TicketSold {
		return void, ctx.Clear("status")
	}

	return void, nil
}

func markAsSold(ctx restate.Context, _ string, _ restate.Void) (void restate.Void, err error) {
	status, err := restate.GetAs[TicketStatus](ctx, "status")
	if err != nil && !errors.Is(err, restate.ErrKeyNotFound) {
		return void, err
	}

	if status == TicketReserved {
		return void, restate.SetAs(ctx, "status", TicketSold)
	}

	return void, nil
}

var (
	TicketService = restate.NewKeyedRouter().
		Handler("reserve", restate.NewKeyedHandler(reserve)).
		Handler("unreserve", restate.NewKeyedHandler(unreserve)).
		Handler("markAsSold", restate.NewKeyedHandler(markAsSold))
)
